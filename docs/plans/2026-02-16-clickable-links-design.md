# F-005: Clickable URLs und File Paths

**Issue:** #31
**Date:** 2026-02-16
**Status:** Approved

## Summary

Make links in terminal output clickable via Ctrl+Click. HTTP(S) URLs open in the system browser, file paths navigate to the file in the integrated sidebar.

## Approach

Use `@xterm/addon-web-links` with a custom regex and handler callback.

### Regex Pattern

Matches two categories:
1. **HTTP(S) URLs** — `https?://[^\s'"\])>]+`
2. **File paths** — paths starting with `./`, `../`, `/`, `C:\` etc., with a file extension, optionally followed by `:line` or `:line:col`

### Handler Logic

```
handler(event, uri):
  if uri starts with http/https:
    BrowserOpenURL(uri)         # Wails runtime — opens system browser
  else:
    dispatch 'navigate-file'    # Sidebar navigates to file
```

### Activation

Links activate on **Ctrl+Click** (like VS Code). Hover with Ctrl shows underline.

## Files Changed

| File | Change |
|------|--------|
| `frontend/package.json` | Add `@xterm/addon-web-links` dependency |
| `frontend/src/lib/terminal.ts` | Import/load WebLinksAddon, define combined regex, export handler setup |
| `frontend/src/components/TerminalPane.svelte` | Wire handler callback (BrowserOpenURL + sidebar navigation event) |

## Edge Cases

- **Relative paths** (`src/foo.ts:42`): Resolved relative to session working directory
- **Windows paths** (`C:\Users\foo\bar.ts`): Covered by regex
- **Non-existent files**: Sidebar attempts navigation; no error shown
