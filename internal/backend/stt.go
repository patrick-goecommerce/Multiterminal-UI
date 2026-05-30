// Package backend — speech-to-text (voice input) transcription.
package backend

import (
	"context"
	"encoding/base64"
	"fmt"
)

// Transcriber turns recorded audio into text.
type Transcriber interface {
	// Transcribe converts raw recorded audio (mime e.g. "audio/webm") to text.
	// lang is an ISO code (e.g. "de") or "auto".
	Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error)
}

// selectTranscriber returns the configured Transcriber.
func (a *AppService) selectTranscriber() (Transcriber, error) {
	switch a.cfg.STT.Provider {
	case "cloud-whisper":
		return &cloudWhisperTranscriber{cfg: a.cfg.STT.Cloud}, nil
	case "whisper-cpp":
		return &whisperCppTranscriber{a: a}, nil
	case "parakeet":
		return &parakeetTranscriber{a: a}, nil
	default:
		return nil, fmt.Errorf("unknown STT provider: %q", a.cfg.STT.Provider)
	}
}

// TranscribeAudio is the Wails binding: base64 audio in, recognized text out.
func (a *AppService) TranscribeAudio(audioB64, mime string) (string, error) {
	audio, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	tr, err := a.selectTranscriber()
	if err != nil {
		return "", err
	}
	return tr.Transcribe(context.Background(), audio, mime, a.cfg.STT.Language)
}

// stubs — replaced in later tasks (8, 9)
type whisperCppTranscriber struct{ a *AppService }

func (t *whisperCppTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	return "", fmt.Errorf("whisper.cpp not yet implemented")
}

type parakeetTranscriber struct{ a *AppService }

func (t *parakeetTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	return "", fmt.Errorf("parakeet not yet implemented")
}
