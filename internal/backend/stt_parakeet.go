// Package backend — local Parakeet (sherpa-onnx) transcriber.
package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type parakeetTranscriber struct{ a *AppService }

// Transcribe converts audio to text using the sherpa-onnx-offline binary with a
// Parakeet ONNX model. Both the binary and model must be placed manually under
// sttEngineDir("parakeet") — auto-download is explicitly deferred (see plan).
//
// Engine layout expected:
//
//	~/.multiterminal/stt/parakeet/sherpa-onnx-offline(.exe)
//	~/.multiterminal/stt/parakeet/model.onnx
func (t *parakeetTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	dir := sttEngineDir("parakeet")
	bin := filepath.Join(dir, binName("sherpa-onnx-offline"))
	model := filepath.Join(dir, "model.onnx")
	if !fileExists(bin) || !fileExists(model) {
		return "", fmt.Errorf("Parakeet-Engine nicht installiert unter %s (sherpa-onnx + Parakeet-Modell). Provider in den Einstellungen wechseln oder Engine bereitstellen.", dir)
	}
	wav, err := toWav16kMono(ctx, audio, mime)
	if err != nil {
		return "", err
	}
	defer os.Remove(wav)
	cmd := commandCtx(ctx, bin, parakeetArgs(model, wav, lang)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sherpa-onnx: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return parseSherpaOutput(string(out)), nil
}

// parakeetArgs builds sherpa-onnx-offline CLI args.
// VERIFY exact flag names against the installed sherpa-onnx version at implementation time —
// flag names differ across releases (e.g. --parakeet vs --model-filename vs positional).
func parakeetArgs(modelPath, wavPath, lang string) []string {
	args := []string{"--parakeet=" + modelPath, wavPath}
	if lang != "" && lang != "auto" {
		args = append(args, "--language", lang)
	}
	return args
}

// parseSherpaOutput extracts the recognized text from sherpa-onnx stdout.
// VERIFY actual output format against the installed sherpa-onnx version at implementation time —
// the current implementation takes the last non-empty line, which matches the
// "single recognized text line at end of output" pattern seen in sherpa-onnx releases,
// but the exact format (prefixes, timestamps) varies across versions.
func parseSherpaOutput(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
