package backend

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSttEngineDir_UnderHome(t *testing.T) {
	p := sttEngineDir("whisper-cpp")
	if !strings.HasSuffix(filepath.ToSlash(p), ".multiterminal/stt/whisper-cpp") {
		t.Errorf("dir = %q, want …/.multiterminal/stt/whisper-cpp", p)
	}
}

func TestWavArgsForLang(t *testing.T) {
	// whisperCppArgs builds CLI args; "auto" must omit -l
	got := strings.Join(whisperCppArgs("/m.bin", "/a.wav", "de"), " ")
	if !strings.Contains(got, "-l de") {
		t.Errorf("expected -l de, got %q", got)
	}
	got2 := strings.Join(whisperCppArgs("/m.bin", "/a.wav", "auto"), " ")
	if strings.Contains(got2, "-l ") {
		t.Errorf("auto should omit -l, got %q", got2)
	}
}

func TestParseWhisperTxt(t *testing.T) {
	raw := "  hallo welt \n"
	if got := parseTranscriptText(raw); got != "hallo welt" {
		t.Errorf("got %q, want 'hallo welt'", got)
	}
}

func TestParakeetArgs_OmitsLangWhenAuto(t *testing.T) {
	a := strings.Join(parakeetArgs("/model", "/a.wav", "auto"), " ")
	if strings.Contains(a, "--language") {
		t.Errorf("auto should omit --language, got %q", a)
	}
}
