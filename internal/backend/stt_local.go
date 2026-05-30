// Package backend — shared helpers for local STT engines (download, wav, exec).
package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
