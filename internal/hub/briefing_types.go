package hub

// GitFacts are the git numbers of a briefing: the turn (start snapshot against
// stop snapshot) and the branch against its target.
type GitFacts struct {
	Branch        string `json:"branch" yaml:"branch"`
	Target        string `json:"target" yaml:"target"`
	OnTarget      bool   `json:"on_target" yaml:"on_target"`
	Detached      bool   `json:"detached" yaml:"detached"`
	NoUpstream    bool   `json:"no_upstream" yaml:"no_upstream"`
	TurnKnown     bool   `json:"turn_known" yaml:"turn_known"`
	TurnFiles     int    `json:"turn_files" yaml:"turn_files"`
	TurnAdded     int    `json:"turn_added" yaml:"turn_added"`
	TurnDeleted   int    `json:"turn_deleted" yaml:"turn_deleted"`
	BranchFiles   int    `json:"branch_files" yaml:"branch_files"`
	BranchAdded   int    `json:"branch_added" yaml:"branch_added"`
	BranchDeleted int    `json:"branch_deleted" yaml:"branch_deleted"`
	Commits       int    `json:"commits" yaml:"commits"`
	Unpushed      int    `json:"unpushed" yaml:"unpushed"`
	Uncommitted   int    `json:"uncommitted" yaml:"uncommitted"`
}

// BriefingPolicy holds the thresholds of a briefing.
type BriefingPolicy struct {
	BigDiffFiles int  `json:"big_diff_files" yaml:"big_diff_files"`
	BigDiffLines int  `json:"big_diff_lines" yaml:"big_diff_lines"`
	CIWatch      bool `json:"ci_watch" yaml:"ci_watch"`
}

const (
	defaultBigDiffFiles = 15
	defaultBigDiffLines = 500
)

// normalized replaces unset (<= 0) thresholds with their defaults.
func (p BriefingPolicy) normalized() BriefingPolicy {
	if p.BigDiffFiles <= 0 {
		p.BigDiffFiles = defaultBigDiffFiles
	}
	if p.BigDiffLines <= 0 {
		p.BigDiffLines = defaultBigDiffLines
	}
	return p
}

// ghRepo names a GitHub repository.
type ghRepo struct {
	Owner string `json:"owner" yaml:"owner"`
	Name  string `json:"name" yaml:"name"`
}

// gitResult is what gitBriefing measured. Head stays on the host (the CI
// watcher compares it to detect a stale result) and is not sent.
type gitResult struct {
	Facts   GitFacts `json:"facts" yaml:"facts"`
	BigDiff bool     `json:"big_diff" yaml:"big_diff"`
	Repo    ghRepo   `json:"repo" yaml:"repo"`
	Head    string   `json:"-" yaml:"-"`
}
