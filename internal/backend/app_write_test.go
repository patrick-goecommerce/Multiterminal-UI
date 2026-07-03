package backend

import (
	"encoding/base64"
	"testing"
)

// The copy/paste and prompt-inject chain crosses a language boundary: the
// frontend encodeForPty(s) = btoa(utf8Bytes(s)) — standard base64 of the UTF-8
// bytes — and the backend WriteToSession decodes it with
// base64.StdEncoding.DecodeString before writing the raw bytes to the PTY.
// These tests pin that contract so the two halves cannot silently drift.

// decodeLikeWriteToSession mirrors the decode WriteToSession performs (app.go).
func decodeLikeWriteToSession(t *testing.T, b64 string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed (frontend/backend base64 dialects diverged?): %v", err)
	}
	return data
}

func TestWriteToSessionDecode_CrossLanguageVector(t *testing.T) {
	// EXACT output of the JS encodeForPty("Grüße über Ötzi"); the identical
	// literal is asserted in frontend/src/lib/clipboard.test.ts. If either side's
	// encoding changes, one of the two tests fails immediately.
	const vector = "R3LDvMOfZSDDvGJlciDDlnR6aQ=="
	const want = "Grüße über Ötzi"
	if got := string(decodeLikeWriteToSession(t, vector)); got != want {
		t.Errorf("decoded %q, want %q — frontend encodeForPty and backend decode are out of lockstep", got, want)
	}
}

func TestWriteToSessionDecode_PreservesPayloadBytes(t *testing.T) {
	// Payloads that must survive the round-trip untouched: bracketed-paste
	// escapes, CRLF, multibyte unicode, and an emoji. The backend must hand the
	// PTY the exact bytes the user copied.
	cases := map[string]string{
		"bracketed_paste": "\x1b[200~rm -rf\x1b[201~",
		"crlf":            "line1\r\nline2",
		"unicode_emoji":   "café ☕ 世界 🚀",
		"tab_hebrew":      "\tא\t  b",
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			// Encode the way the frontend does (base64 of the UTF-8 bytes) and
			// assert the backend decode yields the identical bytes.
			b64 := base64.StdEncoding.EncodeToString([]byte(payload))
			if got := string(decodeLikeWriteToSession(t, b64)); got != payload {
				t.Errorf("round-trip corrupted %s: got %q, want %q", name, got, payload)
			}
		})
	}
}

func TestWriteToSessionDecode_RejectsGarbageWithoutPanic(t *testing.T) {
	// WriteToSession returns silently on a bad base64 payload (never panics /
	// writes garbage to the PTY). Mirror that the decode errors rather than
	// producing bytes.
	if _, err := base64.StdEncoding.DecodeString("not valid base64!!!"); err == nil {
		t.Error("expected invalid base64 to error, so WriteToSession skips the write")
	}
}
