package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func TestCloudWhisper_PostsMultipartAndParsesText(t *testing.T) {
	var gotAuth, gotModel string
	var gotFile bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseMultipartForm(10 << 20)
		gotModel = r.FormValue("model")
		if f, _, err := r.FormFile("file"); err == nil {
			gotFile = true
			io.Copy(io.Discard, f)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"hallo welt"}`))
	}))
	defer srv.Close()

	tr := &cloudWhisperTranscriber{cfg: config.STTCloudSettings{BaseURL: srv.URL, Model: "whisper-1", APIKey: "sk-test"}}
	txt, err := tr.Transcribe(context.Background(), []byte("AUDIODATA"), "audio/webm", "de")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if txt != "hallo welt" {
		t.Errorf("text = %q, want 'hallo welt'", txt)
	}
	if !strings.Contains(gotAuth, "sk-test") {
		t.Errorf("auth header missing key: %q", gotAuth)
	}
	if gotModel != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", gotModel)
	}
	if !gotFile {
		t.Error("no file part sent")
	}
}

func TestCloudWhisper_NoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tr := &cloudWhisperTranscriber{cfg: config.STTCloudSettings{Model: "whisper-1"}}
	if _, err := tr.Transcribe(context.Background(), []byte("x"), "audio/webm", "de"); err == nil {
		t.Fatal("expected error when no API key available")
	}
}
