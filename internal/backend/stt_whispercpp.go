// Package backend — local whisper.cpp transcriber.
package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type whisperCppTranscriber struct{ a *AppService }

// whisperCppModelURL is the on-demand download source for the GGML base model.
// VERIFY current URL at implementation time — external, version-dependent.
const (
	whisperCppModelURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin"
)

// Transcribe records audio to a 16kHz mono WAV, runs whisper-cli, and returns
// the trimmed transcript text. The model is downloaded on first use (~148 MB).
func (t *whisperCppTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	dir := sttEngineDir("whisper-cpp")
	bin := filepath.Join(dir, binName("whisper-cli"))
	model := filepath.Join(dir, "ggml-base.bin")
	if err := t.a.ensureWhisperCpp(ctx, dir, bin, model); err != nil {
		return "", err
	}
	wav, err := toWav16kMono(ctx, audio, mime)
	if err != nil {
		return "", err
	}
	defer os.Remove(wav)
	defer os.Remove(wav + ".txt")

	cmd := commandCtx(ctx, bin, whisperCppArgs(model, wav, lang)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper.cpp: %v: %s", err, strings.TrimSpace(string(b)))
	}
	out, err := os.ReadFile(wav + ".txt")
	if err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}
	return parseTranscriptText(string(out)), nil
}

// ensureWhisperCpp downloads the model on first use; binary must be placed
// manually (auto-download of the binary follows in a later task).
func (a *AppService) ensureWhisperCpp(ctx context.Context, dir, bin, model string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !fileExists(model) {
		a.emitSttDownload("whisper-cpp", 0)
		if err := downloadFile(ctx, whisperCppModelURL, model, func(p int) { a.emitSttDownload("whisper-cpp", p) }); err != nil {
			return fmt.Errorf("modell-download: %w", err)
		}
	}
	if !fileExists(bin) {
		return fmt.Errorf("whisper.cpp-Binary fehlt unter %s — bitte whisper-cli(.exe) dort ablegen (Auto-Download des Binaries folgt)", bin)
	}
	return nil
}

// parseTranscriptText strips surrounding whitespace from whisper.cpp -otxt output.
// whisper.cpp's -nt -otxt flags produce a clean single-line text file.
func parseTranscriptText(s string) string { return strings.TrimSpace(s) }
