package config

import "testing"

func TestDefaultConfig_STTDefaults(t *testing.T) {
	c := DefaultConfig()
	if c.STT.Provider != "cloud-whisper" {
		t.Errorf("provider = %q, want cloud-whisper", c.STT.Provider)
	}
	if c.STT.Language != "de" {
		t.Errorf("language = %q, want de", c.STT.Language)
	}
	if c.STT.Cloud.Model != "whisper-1" {
		t.Errorf("cloud.model = %q, want whisper-1", c.STT.Cloud.Model)
	}
}

func TestLoad_InvalidSTTProviderResetsToCloud(t *testing.T) {
	c := Config{STT: STTSettings{Provider: "bogus"}}
	normalizeSTT(&c)
	if c.STT.Provider != "cloud-whisper" {
		t.Errorf("invalid provider should reset to cloud-whisper, got %q", c.STT.Provider)
	}
}
