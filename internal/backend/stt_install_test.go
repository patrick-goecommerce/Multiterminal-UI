package backend

import (
	"testing"
)

// ---------------------------------------------------------------------------
// assetMatches
// ---------------------------------------------------------------------------

func TestAssetMatches(t *testing.T) {
	tests := []struct {
		name           string
		assetName      string
		mustContain    []string
		mustNotContain []string
		want           bool
	}{
		{
			name:           "whisper-bin-x64 matches",
			assetName:      "whisper-bin-x64.zip",
			mustContain:    []string{"whisper-bin-x64.zip"},
			mustNotContain: []string{"arm", "blas", "cublas"},
			want:           true,
		},
		{
			name:           "whisper-blas-bin excluded",
			assetName:      "whisper-blas-bin-x64.zip",
			mustContain:    []string{"whisper-bin-x64.zip"},
			mustNotContain: []string{"arm", "blas", "cublas"},
			want:           false,
		},
		{
			name:           "sherpa non-streaming asr x64 exe matches",
			assetName:      "sherpa-onnx-non-streaming-asr-x64-v1.13.2.exe",
			mustContain:    []string{"non-streaming-asr", "x64", ".exe"},
			mustNotContain: []string{"arm", "linux", "darwin", "tts", "x86"},
			want:           true,
		},
		{
			name:           "sherpa tts x64 excluded",
			assetName:      "sherpa-onnx-non-streaming-tts-x64-v1.13.2.exe",
			mustContain:    []string{"non-streaming-asr", "x64", ".exe"},
			mustNotContain: []string{"arm", "linux", "darwin", "tts", "x86"},
			want:           false,
		},
		{
			name:           "sherpa x86 excluded",
			assetName:      "sherpa-onnx-non-streaming-asr-x86-v1.13.2.exe",
			mustContain:    []string{"non-streaming-asr", "x64", ".exe"},
			mustNotContain: []string{"arm", "linux", "darwin", "tts", "x86"},
			want:           false,
		},
		{
			name:           "case-insensitive match",
			assetName:      "Whisper-Bin-X64.ZIP",
			mustContain:    []string{"whisper-bin-x64.zip"},
			mustNotContain: []string{},
			want:           true,
		},
		{
			name:           "empty mustContain always matches",
			assetName:      "anything.zip",
			mustContain:    []string{},
			mustNotContain: []string{"arm"},
			want:           true,
		},
		{
			name:           "mustNotContain blocks match",
			assetName:      "whisper-bin-x64.zip",
			mustContain:    []string{},
			mustNotContain: []string{"x64"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assetMatches(tt.assetName, tt.mustContain, tt.mustNotContain)
			if got != tt.want {
				t.Errorf("assetMatches(%q, %v, %v) = %v, want %v",
					tt.assetName, tt.mustContain, tt.mustNotContain, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CheckSttEngine
// ---------------------------------------------------------------------------

func TestCheckSttEngine_CloudAlwaysInstalled(t *testing.T) {
	a := &AppService{}
	s := a.CheckSttEngine("cloud-whisper")
	if !s.Installed {
		t.Error("cloud-whisper should always report installed=true")
	}
	if s.Provider != "cloud-whisper" {
		t.Errorf("provider = %q, want %q", s.Provider, "cloud-whisper")
	}
}

func TestCheckSttEngine_Unknown(t *testing.T) {
	a := &AppService{}
	s := a.CheckSttEngine("unknown-provider")
	if s.Installed {
		t.Error("unknown provider should report installed=false")
	}
	if s.BinPath != "" || s.ModelPath != "" || s.Dir != "" {
		t.Error("unknown provider should have empty paths")
	}
}

func TestCheckSttEngine_WhisperCppPaths(t *testing.T) {
	a := &AppService{}
	s := a.CheckSttEngine("whisper-cpp")
	if s.Provider != "whisper-cpp" {
		t.Errorf("provider = %q, want %q", s.Provider, "whisper-cpp")
	}
	if s.Dir == "" {
		t.Error("Dir should not be empty for whisper-cpp")
	}
	if s.BinPath == "" {
		t.Error("BinPath should not be empty for whisper-cpp")
	}
	if s.ModelPath == "" {
		t.Error("ModelPath should not be empty for whisper-cpp")
	}
	// On a clean CI system these files won't exist, so Installed should be false.
	// We only verify the struct is populated correctly.
	if s.Installed && (!s.BinFound || !s.ModelFound) {
		t.Error("Installed=true requires both BinFound and ModelFound")
	}
}

func TestCheckSttEngine_ParakeetPaths(t *testing.T) {
	a := &AppService{}
	s := a.CheckSttEngine("parakeet")
	if s.Provider != "parakeet" {
		t.Errorf("provider = %q, want %q", s.Provider, "parakeet")
	}
	if s.Dir == "" {
		t.Error("Dir should not be empty for parakeet")
	}
	if s.BinPath == "" {
		t.Error("BinPath should not be empty for parakeet")
	}
	if s.ModelPath == "" {
		t.Error("ModelPath should not be empty for parakeet")
	}
}
