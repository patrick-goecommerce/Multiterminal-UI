package config

import (
	"encoding/json"
	"testing"
)

func TestSavedPane_WorktreeFieldsRoundtrip(t *testing.T) {
	p := SavedPane{Name: "x", WorktreePath: `D:\repos\Foo.mt-worktrees\a`, WorktreeBranch: "terminal/a", TargetBranch: "alpha-main"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got SavedPane
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.WorktreePath != p.WorktreePath || got.WorktreeBranch != p.WorktreeBranch || got.TargetBranch != p.TargetBranch {
		t.Errorf("roundtrip lost fields: %+v", got)
	}
}
