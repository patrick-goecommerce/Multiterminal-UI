package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func TestSelectTranscriber_ByProvider(t *testing.T) {
	a := &AppService{cfg: config.Config{STT: config.STTSettings{Provider: "cloud-whisper", Cloud: config.STTCloudSettings{Model: "whisper-1"}}}}
	tr, err := a.selectTranscriber()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, ok := tr.(*cloudWhisperTranscriber); !ok {
		t.Fatalf("got %T, want *cloudWhisperTranscriber", tr)
	}
}

func TestSelectTranscriber_Unknown(t *testing.T) {
	a := &AppService{cfg: config.Config{STT: config.STTSettings{Provider: "nope"}}}
	if _, err := a.selectTranscriber(); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

var _ Transcriber = (*cloudWhisperTranscriber)(nil)
