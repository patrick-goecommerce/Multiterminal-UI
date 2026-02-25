package backend

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// BrowseForAudioFile opens a native file picker for audio files.
func (a *App) BrowseForAudioFile() string {
	path, err := wailsrt.OpenFileDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title: "Audio-Datei auswählen",
		Filters: []wailsrt.FileFilter{
			{DisplayName: "Audio Files (*.mp3, *.wav, *.ogg, *.flac)", Pattern: "*.mp3;*.wav;*.ogg;*.flac"},
		},
	})
	if err != nil || path == "" {
		return ""
	}
	return path
}
