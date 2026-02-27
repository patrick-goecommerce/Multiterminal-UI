# Multiterminal Claude Code hook handler
# Reads hook event from stdin, appends JSONL line to hooks directory.
# Called by Claude Code for: PreToolUse, PostToolUse, PostToolUseFailure,
#   PermissionRequest, Notification, Stop, SessionEnd

param([string]$EventType)

try {
    # Read stdin JSON from Claude Code
    $stdin = [Console]::In.ReadToEnd()
    $data = $stdin | ConvertFrom-Json -ErrorAction Stop

    # Determine hooks directory
    $hooksDir = Join-Path $env:APPDATA "Multiterminal\hooks"
    if (-not (Test-Path $hooksDir)) {
        New-Item -ItemType Directory -Path $hooksDir -Force | Out-Null
    }

    # Build JSONL payload
    $sessionId = if ($data.session_id) { $data.session_id } else { "unknown" }
    $mtSessionId = if ($env:MULTITERMINAL_SESSION_ID) { [int]$env:MULTITERMINAL_SESSION_ID } else { 0 }
    $toolName = if ($data.tool_name) { $data.tool_name } else { "" }
    $message = if ($data.message) { $data.message } else { "" }
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()

    $payload = [ordered]@{
        ts         = $ts
        event      = $EventType
        session_id = $sessionId
        mt_id      = $mtSessionId
        tool       = $toolName
        message    = $message
    }
    $line = $payload | ConvertTo-Json -Compress

    # Append to JSONL file (one file per Claude session)
    $file = Join-Path $hooksDir "$sessionId.jsonl"
    Add-Content -Path $file -Value $line -Encoding UTF8 -NoNewline
    Add-Content -Path $file -Value "`n" -Encoding UTF8 -NoNewline
} catch {
    # Silent failure — never block Claude Code
    exit 0
}
exit 0
