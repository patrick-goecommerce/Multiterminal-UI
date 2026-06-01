// Package backend — on-demand installer for local STT engines.
package backend

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SttEngineStatus tells the frontend whether a local engine is installed.
type SttEngineStatus struct {
	Provider   string `json:"provider" yaml:"provider"`
	Dir        string `json:"dir" yaml:"dir"`
	BinPath    string `json:"bin_path" yaml:"bin_path"`
	ModelPath  string `json:"model_path" yaml:"model_path"`
	BinFound   bool   `json:"bin_found" yaml:"bin_found"`
	ModelFound bool   `json:"model_found" yaml:"model_found"`
	Installed  bool   `json:"installed" yaml:"installed"`
}

// CheckSttEngine reports whether the binary and model for a local engine
// are present on disk. Cloud-whisper always reports installed=true.
func (a *AppService) CheckSttEngine(provider string) SttEngineStatus {
	s := SttEngineStatus{Provider: provider}
	switch provider {
	case "cloud-whisper":
		s.Installed = true
		return s
	case "whisper-cpp":
		s.Dir = sttEngineDir("whisper-cpp")
		s.BinPath = filepath.Join(s.Dir, binName("whisper-cli"))
		s.ModelPath = filepath.Join(s.Dir, "ggml-base.bin")
	case "parakeet":
		s.Dir = sttEngineDir("parakeet")
		s.BinPath = filepath.Join(s.Dir, binName("sherpa-onnx-offline"))
		s.ModelPath = filepath.Join(s.Dir, "model.onnx")
	default:
		return s
	}
	s.BinFound = fileExists(s.BinPath)
	s.ModelFound = fileExists(s.ModelPath)
	s.Installed = s.BinFound && s.ModelFound
	return s
}

// InstallSttEngine downloads + extracts the binary and (if missing) the model
// for the named local engine. Emits "stt:download" events with phase info.
func (a *AppService) InstallSttEngine(provider string) error {
	ctx := context.Background()
	switch provider {
	case "whisper-cpp":
		return a.installWhisperCpp(ctx)
	case "parakeet":
		return a.installParakeet(ctx)
	default:
		return fmt.Errorf("unsupported STT engine: %q", provider)
	}
}

// ---------------------------------------------------------------------------
// GitHub Releases helper
// ---------------------------------------------------------------------------

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// findLatestAsset queries GitHub's API for owner/repo's latest release and
// returns the first asset whose Name contains all of mustContain and none of
// mustNotContain. Empty match → error.
func findLatestAsset(ctx context.Context, owner, repo string, mustContain, mustNotContain []string) (ghAsset, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ghAsset{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ghAsset{}, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ghAsset{}, fmt.Errorf("github api %s: status %d", url, resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghAsset{}, fmt.Errorf("github api parse: %w", err)
	}
	for _, ast := range rel.Assets {
		if assetMatches(ast.Name, mustContain, mustNotContain) {
			return ast, nil
		}
	}
	return ghAsset{}, fmt.Errorf("no asset matches in %s/%s@%s (need %v, exclude %v)",
		owner, repo, rel.TagName, mustContain, mustNotContain)
}

func assetMatches(name string, mustContain, mustNotContain []string) bool {
	lower := strings.ToLower(name)
	for _, w := range mustContain {
		if !strings.Contains(lower, strings.ToLower(w)) {
			return false
		}
	}
	for _, w := range mustNotContain {
		if strings.Contains(lower, strings.ToLower(w)) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Archive extractors
// ---------------------------------------------------------------------------

// extractZip extracts entries from zipPath into destDir.
// keep returns the destination filename for a given zip entry (or "" to skip).
func extractZip(zipPath, destDir string, keep func(name string) string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		outName := keep(f.Name)
		if outName == "" {
			continue
		}
		outPath := filepath.Join(destDir, outName)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("zip open %s: %w", f.Name, err)
		}
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return fmt.Errorf("zip extract %s: %w", f.Name, err)
		}
		out.Close()
		rc.Close()
		os.Chmod(outPath, 0o755) //nolint:errcheck // best-effort
	}
	return nil
}

// extractTarBz2 extracts entries from a tar.bz2 file into destDir.
func extractTarBz2(tarPath, destDir string, keep func(name string) string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar.bz2: %w", err)
	}
	defer f.Close()
	tr := tar.NewReader(bzip2.NewReader(f))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar.bz2 read: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		outName := keep(hdr.Name)
		if outName == "" {
			continue
		}
		outPath := filepath.Join(destDir, outName)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("tar.bz2 extract %s: %w", hdr.Name, err)
		}
		out.Close()
		os.Chmod(outPath, 0o755) //nolint:errcheck // best-effort
	}
	return nil
}

// ---------------------------------------------------------------------------
// whisper.cpp installer
// ---------------------------------------------------------------------------

// installWhisperCpp downloads whisper-bin-x64.zip from the ggml-org/whisper.cpp
// GitHub release and extracts whisper-cli.exe + DLLs. Model is downloaded from
// HuggingFace (~148 MB). Windows-only for v1.
func (a *AppService) installWhisperCpp(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("Auto-Installer derzeit nur für Windows; manuell installieren")
	}
	dir := sttEngineDir("whisper-cpp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// BIN phase — whisper-bin-x64.zip contains whisper-cli.exe + DLLs.
	bin := filepath.Join(dir, binName("whisper-cli"))
	if !fileExists(bin) {
		a.emitSttPhase("whisper-cpp", "binary", 0)
		asset, err := findLatestAsset(ctx, "ggml-org", "whisper.cpp",
			[]string{"whisper-bin-x64.zip"},
			[]string{"arm", "blas", "cublas"},
		)
		if err != nil {
			return fmt.Errorf("whisper.cpp binary lookup: %w", err)
		}
		tmpZip := filepath.Join(dir, "whisper-bin.zip.part")
		if err := downloadFile(ctx, asset.URL, tmpZip,
			func(p int) { a.emitSttPhase("whisper-cpp", "binary", p) }); err != nil {
			return fmt.Errorf("whisper.cpp binary download: %w", err)
		}
		// Extract whisper-cli.exe (or main.exe — older releases) + all .dll files.
		err = extractZip(tmpZip, dir, func(name string) string {
			base := filepath.Base(name)
			lower := strings.ToLower(base)
			switch {
			case lower == "whisper-cli.exe", lower == "main.exe":
				return binName("whisper-cli") // normalize to whisper-cli.exe
			case strings.HasSuffix(lower, ".dll"):
				return base
			}
			return ""
		})
		os.Remove(tmpZip) //nolint:errcheck
		if err != nil {
			return err
		}
		if !fileExists(bin) {
			return fmt.Errorf("whisper.cpp ZIP enthielt weder whisper-cli.exe noch main.exe")
		}
		a.emitSttPhase("whisper-cpp", "binary", 100)
	}

	// MODEL phase — ggml-base.bin from HuggingFace (~148 MB).
	model := filepath.Join(dir, "ggml-base.bin")
	if !fileExists(model) {
		a.emitSttPhase("whisper-cpp", "model", 0)
		if err := downloadFile(ctx, whisperCppModelURL, model,
			func(p int) { a.emitSttPhase("whisper-cpp", "model", p) }); err != nil {
			return fmt.Errorf("whisper.cpp Modell-Download: %w", err)
		}
		a.emitSttPhase("whisper-cpp", "model", 100)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Parakeet installer
// ---------------------------------------------------------------------------

// parakeetModelURL is the pinned source for the Parakeet TDT int8 ONNX model.
// VERIFY at implementation time: curl -sI <url> → HTTP 302 means the file exists.
const parakeetModelURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2"

// installParakeet downloads the sherpa-onnx non-streaming ASR binary (standalone
// Windows exe) and the Parakeet ONNX model. Windows-only for v1.
//
// sherpa-onnx v1.13.2+ ships sherpa-onnx-non-streaming-asr-x64-vX.Y.Z.exe as a
// standalone statically-linked executable — no DLLs, no tar.bz2.
// We download it directly, rename to sherpa-onnx-offline.exe for consistency
// with the parakeetTranscriber's expected binary name.
func (a *AppService) installParakeet(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("Auto-Installer derzeit nur für Windows; manuell installieren")
	}
	dir := sttEngineDir("parakeet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// BIN phase — sherpa-onnx non-streaming ASR standalone exe.
	bin := filepath.Join(dir, binName("sherpa-onnx-offline"))
	if !fileExists(bin) {
		a.emitSttPhase("parakeet", "binary", 0)
		asset, err := findLatestAsset(ctx, "k2-fsa", "sherpa-onnx",
			[]string{"non-streaming-asr", "x64", ".exe"},
			[]string{"arm", "linux", "darwin", "tts", "x86"},
		)
		if err != nil {
			return fmt.Errorf("sherpa-onnx Binary konnte nicht gefunden werden: %w (manuell von https://github.com/k2-fsa/sherpa-onnx/releases installieren)", err)
		}
		tmp := filepath.Join(dir, "sherpa-offline.exe.part")
		if err := downloadFile(ctx, asset.URL, tmp,
			func(p int) { a.emitSttPhase("parakeet", "binary", p) }); err != nil {
			os.Remove(tmp) //nolint:errcheck
			return fmt.Errorf("sherpa-onnx Binary konnte nicht gefunden werden: %w (manuell von https://github.com/k2-fsa/sherpa-onnx/releases installieren)", err)
		}
		if err := os.Rename(tmp, bin); err != nil {
			os.Remove(tmp) //nolint:errcheck
			return fmt.Errorf("sherpa-onnx Archiv enthielt nicht sherpa-onnx-offline.exe")
		}
		os.Chmod(bin, 0o755) //nolint:errcheck
		if !fileExists(bin) {
			return fmt.Errorf("sherpa-onnx Archiv enthielt nicht sherpa-onnx-offline.exe")
		}
		a.emitSttPhase("parakeet", "binary", 100)
	}

	// MODEL phase — Parakeet TDT int8 ONNX model.
	model := filepath.Join(dir, "model.onnx")
	if !fileExists(model) {
		a.emitSttPhase("parakeet", "model", 0)
		tmp := filepath.Join(dir, "parakeet-model.tar.bz2.part")
		if err := downloadFile(ctx, parakeetModelURL, tmp,
			func(p int) { a.emitSttPhase("parakeet", "model", p) }); err != nil {
			os.Remove(tmp) //nolint:errcheck
			return fmt.Errorf("Parakeet-Modell-Download: %w", err)
		}
		// Look for *.onnx — normalize to model.onnx. Keep tokens.txt if present.
		err := extractTarBz2(tmp, dir, func(name string) string {
			base := filepath.Base(name)
			lower := strings.ToLower(base)
			switch {
			case strings.HasSuffix(lower, ".onnx"):
				return "model.onnx"
			case lower == "tokens.txt":
				return "tokens.txt"
			}
			return ""
		})
		os.Remove(tmp) //nolint:errcheck
		if err != nil {
			return err
		}
		if !fileExists(model) {
			return fmt.Errorf("Parakeet-Archiv enthielt keine .onnx-Modelldatei")
		}
		a.emitSttPhase("parakeet", "model", 100)
	}
	return nil
}
