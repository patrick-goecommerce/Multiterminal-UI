package backend

import (
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
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
