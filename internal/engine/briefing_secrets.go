package engine

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SecretFinding represents a detected secret — NEVER stores the actual value.
type SecretFinding struct {
	Type    string `json:"type"`    // "AWS_KEY" | "GITHUB_TOKEN" | "PRIVATE_KEY" | "DB_CREDENTIALS" | "STRIPE_KEY" | "GENERIC_SECRET"
	File    string `json:"file"`
	Line    int    `json:"line"`
	Preview string `json:"preview"` // redacted, e.g. "STRIPE_SK=sk_live_****"
}

// secretPattern pairs a type label with its detection regex.
type secretPattern struct {
	Type    string
	Pattern *regexp.Regexp
}

// secretPatterns defines the patterns we scan for.
// Order matters: more specific patterns before GENERIC_SECRET.
var secretPatterns = []secretPattern{
	{"AWS_KEY", regexp.MustCompile(`(?i)(AKIA[0-9A-Z]{16})`)},
	{"GITHUB_TOKEN", regexp.MustCompile(`(?i)(ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{82})`)},
	{"STRIPE_KEY", regexp.MustCompile(`(?i)(sk_live_[a-zA-Z0-9]{24,})`)},
	{"PRIVATE_KEY", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
	{"DB_CREDENTIALS", regexp.MustCompile(`(?i)(postgres|mysql|mongodb)://[^:]+:[^@]+@`)},
	{"GENERIC_SECRET", regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key|access[_-]?token)\s*[:=]\s*["']?[a-zA-Z0-9/+]{20,}`)},
}

// redactValue masks the actual secret value.
// Shows at most 10 characters of the match followed by "****".
func redactValue(line, match string) string {
	idx := strings.Index(line, match)
	if idx < 0 {
		return "****"
	}
	prefix := line[:idx]
	if len(match) > 10 {
		return prefix + match[:10] + "****"
	}
	return prefix + "****"
}

// openFile is a thin wrapper for filesystem access.
func openFile(path string) (*os.File, error) {
	return os.Open(path)
}

// scanSecrets scans changed files for secret patterns.
// NEVER stores actual secret values — only type, file, line, redacted preview.
func scanSecrets(workDir string, files []string) []SecretFinding {
	var findings []SecretFinding
	for _, f := range files {
		full := filepath.Join(workDir, f)
		ff := scanFileForSecrets(full, f)
		findings = append(findings, ff...)
	}
	return findings
}

// scanFileForSecrets reads a single file and checks each line.
func scanFileForSecrets(fullPath, relPath string) []SecretFinding {
	var findings []SecretFinding

	file, err := openFile(fullPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, sp := range secretPatterns {
			match := sp.Pattern.FindString(line)
			if match == "" {
				continue
			}
			findings = append(findings, SecretFinding{
				Type:    sp.Type,
				File:    relPath,
				Line:    lineNum,
				Preview: redactValue(line, match),
			})
			break // one finding per line is enough
		}
	}
	return findings
}
