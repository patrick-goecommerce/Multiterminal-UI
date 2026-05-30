// Package backend — shared helpers for local STT engines (download, wav, exec).
package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// sttEngineDir returns the on-demand storage dir for an engine's binary+model.
func sttEngineDir(engine string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".multiterminal", "stt", engine)
}

// ffmpegPath returns the ffmpeg executable path, or "" if not found in PATH.
func ffmpegPath() string {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return p
}

// toWav16kMono converts recorded audio bytes to a temp 16kHz mono WAV file via
// ffmpeg, returning the wav path (caller deletes). Requires ffmpeg in PATH.
func toWav16kMono(ctx context.Context, audio []byte, mime string) (string, error) {
	ff := ffmpegPath()
	if ff == "" {
		return "", fmt.Errorf("ffmpeg nicht im PATH gefunden (für lokale STT-Engines benötigt)")
	}
	in, err := os.CreateTemp("", "mtui-stt-in*"+extForMime(mime))
	if err != nil {
		return "", err
	}
	defer os.Remove(in.Name())
	if _, err := in.Write(audio); err != nil {
		in.Close()
		return "", err
	}
	in.Close()
	out := in.Name() + ".wav"
	cmd := exec.CommandContext(ctx, ff, "-y", "-i", in.Name(), "-ar", "16000", "-ac", "1", "-f", "wav", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		os.Remove(out) // best-effort cleanup; out may not exist if ffmpeg failed early
		return "", fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return out, nil
}

// whisperCppArgs builds the whisper.cpp CLI args (no executable).
func whisperCppArgs(modelPath, wavPath, lang string) []string {
	args := []string{"-m", modelPath, "-f", wavPath, "-nt", "-otxt", "-of", wavPath}
	if lang != "" && lang != "auto" {
		args = append(args, "-l", lang)
	}
	return args
}

// emitSttDownload reports model/binary download progress to the frontend.
func (a *AppService) emitSttDownload(engine string, pct int) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("stt:download", map[string]interface{}{"engine": engine, "pct": pct})
}

// binName appends ".exe" on Windows, otherwise returns base unchanged.
func binName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// commandCtx is a thin wrapper around exec.CommandContext to allow future
// test-time substitution without changing call sites.
func commandCtx(ctx context.Context, bin string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, bin, args...)
}

// downloadFile streams url to dst, reporting integer percent via progress.
// Uses an atomic .part + rename pattern to avoid partial files on error.
func downloadFile(ctx context.Context, url, dst string, progress func(pct int)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	var written int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			written += int64(n)
			if total > 0 && progress != nil {
				progress(int(written * 100 / total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	f.Close()
	return os.Rename(tmp, dst)
}
