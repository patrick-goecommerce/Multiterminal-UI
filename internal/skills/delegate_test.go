package skills

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Claude Code only loads a skill whose SKILL.md opens with front matter that
// names it and says when to use it. A typo there and the skill silently never
// triggers.
func TestDelegateSkill_HasFrontMatterClaudeCanLoad(t *testing.T) {
	text := string(DelegateSkill())
	if !strings.HasPrefix(text, "---\n") {
		t.Fatal("SKILL.md must start with YAML front matter")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		t.Fatal("front matter is not closed")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		if key, value, ok := strings.Cut(line, ": "); ok {
			fields[key] = value
		}
	}
	if fields["name"] != DelegateSkillName {
		t.Errorf("front matter names %q, want %q", fields["name"], DelegateSkillName)
	}
	desc := fields["description"]
	if desc == "" {
		t.Fatal("front matter has no description")
	}
	// Claude Code caps the description; beyond it the skill is not listed.
	if len(desc) > 1024 {
		t.Errorf("description is %d characters, the limit is 1024", len(desc))
	}
	if !strings.Contains(desc, "MULTITERMINAL_SESSION_ID") {
		t.Error("the description must say the skill only applies inside MTUI")
	}
	if !bytes.Contains(DelegateSkill(), delegateMarker) {
		t.Error("the embedded skill lacks the marker MTUI recognises it by")
	}
}

// The skill promises tools and commands. Each of them has to exist, or an
// agent that follows it hits an unknown tool.
func TestDelegateSkill_NamesOnlyToolsThatExist(t *testing.T) {
	text := string(DelegateSkill())
	for _, name := range []string{
		"open_session", "send_input", "wait_for_agent", "read_output", "close_session", "list_sessions",
		"mt new", "mt wait", "mt read", "mt send", "mt keys", "mt kill", "mt ls", "mt hub",
	} {
		if !strings.Contains(text, name) {
			t.Errorf("skill no longer mentions %q; update this test if that was deliberate", name)
		}
	}
}

func TestInstallDelegateSkill_WritesAndThenLeavesACurrentFileAlone(t *testing.T) {
	dir := t.TempDir()
	changed, err := InstallDelegateSkill(dir)
	if err != nil || !changed {
		t.Fatalf("first install = (%v, %v), want (true, nil)", changed, err)
	}
	path := delegateSkillPath(dir)
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, DelegateSkill()) {
		t.Fatalf("installed file differs from the embedded skill (err %v)", err)
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if changed, err := InstallDelegateSkill(dir); err != nil || changed {
		t.Errorf("second install = (%v, %v), want (false, nil)", changed, err)
	}
	if info, _ := os.Stat(path); !info.ModTime().Equal(old) {
		t.Error("a current file was rewritten")
	}
}

// Git or an editor on Windows may turn the file into CRLF. That is still our
// file, and still current.
func TestInstallDelegateSkill_ToleratesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := delegateSkillPath(dir)
	crlf := bytes.ReplaceAll(DelegateSkill(), []byte("\n"), []byte("\r\n"))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, crlf, 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := InstallDelegateSkill(dir); err != nil || changed {
		t.Errorf("install over a CRLF copy = (%v, %v), want (false, nil)", changed, err)
	}
}

func TestInstallDelegateSkill_UpdatesAnOlderManagedCopy(t *testing.T) {
	dir := t.TempDir()
	path := delegateSkillPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "---\nname: mtui-delegate\ndescription: old\n---\n<!-- mtui:managed -->\nold text\n"
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := InstallDelegateSkill(dir); err != nil || !changed {
		t.Fatalf("install over a stale copy = (%v, %v), want (true, nil)", changed, err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, DelegateSkill()) {
		t.Error("stale copy was not replaced")
	}
}

// Deleting the marker line is how a user keeps their own version.
func TestInstallAndRemove_LeaveAUserOwnedCopyAlone(t *testing.T) {
	dir := t.TempDir()
	path := delegateSkillPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "---\nname: mtui-delegate\ndescription: my own\n---\nmy rules\n"
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := InstallDelegateSkill(dir); err != nil || changed {
		t.Errorf("install over a user copy = (%v, %v), want (false, nil)", changed, err)
	}
	if removed, err := RemoveDelegateSkill(dir); err != nil || removed {
		t.Errorf("remove of a user copy = (%v, %v), want (false, nil)", removed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != mine {
		t.Error("the user's copy was changed")
	}
}

func TestRemoveDelegateSkill_RemovesTheManagedCopyAndItsDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallDelegateSkill(dir); err != nil {
		t.Fatal(err)
	}
	if removed, err := RemoveDelegateSkill(dir); err != nil || !removed {
		t.Fatalf("remove = (%v, %v), want (true, nil)", removed, err)
	}
	if _, err := os.Stat(filepath.Dir(delegateSkillPath(dir))); !os.IsNotExist(err) {
		t.Error("the empty skill directory was left behind")
	}
	if removed, err := RemoveDelegateSkill(dir); err != nil || removed {
		t.Errorf("second remove = (%v, %v), want (false, nil)", removed, err)
	}
}
