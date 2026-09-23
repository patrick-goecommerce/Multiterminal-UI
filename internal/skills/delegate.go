package skills

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The delegation skill.
//
// The templates in this package are text blocks MTUI writes into a project's
// CLAUDE.md. This one is different: a Claude Code Agent Skill, a SKILL.md
// under ~/.claude/skills that Claude loads on its own when the description
// matches the task. It teaches an agent in a pane how to hand work to another
// pane through the mtui MCP tools or `mt`, which is the third of the three
// faces the daemon spec asks for (window, CLI, agent skill).

//go:embed delegate/SKILL.md
var delegateSkill []byte

// DelegateSkillName is the directory the skill lives in under skills/.
const DelegateSkillName = "mtui-delegate"

// delegateMarker identifies a SKILL.md MTUI still manages. It is matched as a
// substring rather than by comparing the whole file: a CRLF round trip
// through an editor or git on Windows would otherwise make MTUI lose track of
// its own file (the same reason the generated CLAUDE.local.md is recognised by
// its marker line).
var delegateMarker = []byte("<!-- mtui:managed")

// DelegateSkill returns the skill's content.
func DelegateSkill() []byte { return bytes.Clone(delegateSkill) }

func delegateSkillPath(claudeDir string) string {
	return filepath.Join(claudeDir, "skills", DelegateSkillName, "SKILL.md")
}

// InstallDelegateSkill writes the skill to <claudeDir>/skills/mtui-delegate.
// It reports whether it wrote anything.
//
// A file that is already current is left untouched, so the modification time
// only moves when the content does. A file without the marker belongs to the
// user, who deleted the marker line to keep their own edits, and is never
// overwritten.
func InstallDelegateSkill(claudeDir string) (bool, error) {
	path := delegateSkillPath(claudeDir)
	current, err := os.ReadFile(path)
	switch {
	case err == nil:
		if !bytes.Contains(current, delegateMarker) {
			return false, nil
		}
		if bytes.Equal(normalizeNewlines(current), normalizeNewlines(delegateSkill)) {
			return false, nil
		}
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, delegateSkill, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// RemoveDelegateSkill deletes the skill again, but only while MTUI still
// manages it. It is what turning agent control off does: a skill that points
// at tools which are gone only sends the agent down a dead end. It reports
// whether it removed anything.
func RemoveDelegateSkill(claudeDir string) (bool, error) {
	path := delegateSkillPath(claudeDir)
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Contains(current, delegateMarker) {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove %s: %w", path, err)
	}
	// The directory only held the skill. Leave it if the user put anything
	// else there.
	_ = os.Remove(filepath.Dir(path))
	return true, nil
}

func normalizeNewlines(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}
