// Package backend — cloud Whisper-compatible transcription.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

type cloudWhisperTranscriber struct{ cfg config.STTCloudSettings }

func (t *cloudWhisperTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	key := t.cfg.APIKey
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		return "", fmt.Errorf("kein API-Key gesetzt (Settings → Spracheingabe, oder $OPENAI_API_KEY)")
	}
	base := strings.TrimRight(t.cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := t.cfg.Model
	if model == "" {
		model = "whisper-1"
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "audio"+extForMime(mime))
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	_ = mw.WriteField("model", model)
	if lang != "" && lang != "auto" {
		_ = mw.WriteField("language", lang)
	}
	_ = mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcription failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// extForMime maps a recording mime to a file extension the API understands.
func extForMime(mime string) string {
	switch {
	case strings.Contains(mime, "webm"):
		return ".webm"
	case strings.Contains(mime, "ogg"):
		return ".ogg"
	case strings.Contains(mime, "mp4"), strings.Contains(mime, "mp4a"), strings.Contains(mime, "m4a"):
		return ".m4a"
	case strings.Contains(mime, "wav"):
		return ".wav"
	default:
		return ".webm"
	}
}
