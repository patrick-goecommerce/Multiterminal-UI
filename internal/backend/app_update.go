package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ApplyUpdate downloads, verifies, and applies the latest release for the
// configured channel, then restarts mtui. It re-resolves the release fresh
// rather than trusting a stale UpdateInfo blob round-tripped from the
// frontend, and refuses to proceed unless the download matches its published
// SHA256 checksum.
//
// Known, accepted limitation (see issue #165): the checksum is produced by
// the same CI pipeline that builds the exe, so it guards against transit
// corruption, not a compromised release pipeline/account. Proper mitigation
// (Authenticode signing or GPG-signed checksums with an embedded public key)
// is deferred for v1.
func (a *AppService) ApplyUpdate() error {
	channel := a.cfg.UpdateChannel
	if channel == "" {
		channel = "stable"
	}

	release, err := resolveLatestRelease(channel)
	if err != nil {
		return fmt.Errorf("release lookup failed: %w", err)
	}

	portable := isPortableBuild()
	assetName, assetURL, checksumURL := resolveAsset(release, portable)
	if assetURL == "" {
		return fmt.Errorf("no matching release asset found")
	}
	if checksumURL == "" {
		return fmt.Errorf("no checksum published for %s, refusing to apply", assetName)
	}

	// Random per-invocation directory (not a version-derived name) so the
	// downloaded/verified file isn't at a predictable path another process
	// running as the same user could race against before it's used.
	workDir, err := os.MkdirTemp("", "mtui-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	newExePath := filepath.Join(workDir, assetName)
	checksumPath := filepath.Join(workDir, assetName+".sha256")

	if err := downloadUpdateAsset(assetURL, newExePath); err != nil {
		os.RemoveAll(workDir)
		return fmt.Errorf("download failed: %w", err)
	}
	if err := downloadUpdateAsset(checksumURL, checksumPath); err != nil {
		os.RemoveAll(workDir)
		return fmt.Errorf("checksum download failed: %w", err)
	}
	if err := verifyChecksum(newExePath, checksumPath); err != nil {
		os.RemoveAll(workDir)
		return err
	}

	ownExe, err := os.Executable()
	if err != nil {
		os.RemoveAll(workDir)
		return fmt.Errorf("failed to resolve own executable path: %w", err)
	}

	scriptPath := filepath.Join(workDir, "apply.ps1")
	logPath := filepath.Join(workDir, "apply.log")
	script := buildUpdateScript(os.Getpid(), newExePath, checksumPath, ownExe, logPath, portable)
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		os.RemoveAll(workDir)
		return fmt.Errorf("failed to write update script: %w", err)
	}

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(workDir)
		return fmt.Errorf("failed to launch updater: %w", err)
	}

	// Give the frontend a moment to show a "restarting" message before the
	// window disappears; the relauncher script is already waiting on our PID
	// and will proceed once this process actually exits (releasing the file
	// lock on ownExe).
	time.AfterFunc(1500*time.Millisecond, func() { os.Exit(0) })

	return nil
}

// downloadFile streams the contents of url to path.
func downloadUpdateAsset(url, path string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// verifyChecksum compares the SHA256 of path against the expected hash parsed
// from the first whitespace-separated token of checksumPath (sha256sum
// output format: "<hash>  <filename>").
func verifyChecksum(path, checksumPath string) error {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("failed to read checksum file: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return fmt.Errorf("empty checksum file")
	}
	expected := strings.ToLower(fields[0])

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open downloaded file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash downloaded file: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))

	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// psQuote wraps s in single quotes for safe embedding in a generated
// PowerShell script, escaping any embedded single quotes.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// buildUpdateScript generates the relauncher PowerShell script. It:
//  1. waits for this process (pid) to exit, releasing the file lock on ownExe;
//  2. re-verifies the checksum immediately before use, closing the TOCTOU
//     window between Go's earlier verification and actual execution;
//  3. applies the update (portable: backs up and overwrites the exe;
//     installed: runs the downloaded installer silently);
//  4. relaunches and checks the new process survives its first few seconds,
//     rolling back to the backup if not (portable variant only — the
//     installed variant has no previous installer cached to roll back to,
//     so it's best-effort + logged only);
//  5. cleans up its own temp files, including itself.
func buildUpdateScript(pid int, newExe, checksumFile, ownExe, logFile string, portable bool) string {
	var apply string
	if portable {
		backup := ownExe + ".bak"
		apply = fmt.Sprintf(`Copy-Item -Path %s -Destination %s -Force
    Copy-Item -Path %s -Destination %s -Force
    $proc = Start-Process -FilePath %s -PassThru
    Start-Sleep -Seconds 3
    if (-not (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) {
        Log "new process exited within 3s, rolling back"
        Copy-Item -Path %s -Destination %s -Force
        Start-Process -FilePath %s
    } else {
        Remove-Item -Path %s -Force -ErrorAction SilentlyContinue
    }`,
			psQuote(ownExe), psQuote(backup),
			psQuote(newExe), psQuote(ownExe),
			psQuote(ownExe),
			psQuote(backup), psQuote(ownExe),
			psQuote(ownExe),
			psQuote(backup))
	} else {
		apply = fmt.Sprintf(`Start-Process -FilePath %s -ArgumentList '/VERYSILENT','/NORESTART','/SUPPRESSMSGBOXES' -Wait
    $proc = Start-Process -FilePath %s -PassThru
    Start-Sleep -Seconds 3
    if (-not (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) {
        Log "new process exited within 3s after silent install"
    }`,
			psQuote(newExe), psQuote(ownExe))
	}

	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
function Log($msg) { Add-Content -Path %s -Value "$(Get-Date -Format o) $msg" -ErrorAction SilentlyContinue }

Wait-Process -Id %d -ErrorAction SilentlyContinue

try {
    $expected = ((Get-Content -Path %s -Raw).Trim() -split '\s+')[0].ToLower()
    $actual = (Get-FileHash -Path %s -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $expected) {
        Log "checksum mismatch on re-verify: expected=$expected actual=$actual"
        exit 1
    }

    %s
} catch {
    Log "update failed: $_"
} finally {
    Remove-Item -Path %s -Force -ErrorAction SilentlyContinue
    Remove-Item -Path %s -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $MyInvocation.MyCommand.Path -Force -ErrorAction SilentlyContinue
}
`,
		psQuote(logFile),
		pid,
		psQuote(checksumFile),
		psQuote(newExe),
		apply,
		psQuote(newExe),
		psQuote(checksumFile))
}
