package hub

import "strings"

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

// PRFacts is what GitHub knows about the pull request of the branch.
type PRFacts struct {
	Number       int    `json:"number" yaml:"number"`
	URL          string `json:"url" yaml:"url"`
	State        string `json:"state" yaml:"state"`
	Base         string `json:"base" yaml:"base"`
	HeadOID      string `json:"head_oid" yaml:"head_oid"`
	Review       string `json:"review" yaml:"review"`
	Checks       string `json:"checks" yaml:"checks"`
	FailedCheck  string `json:"failed_check" yaml:"failed_check"`
	Draft        bool   `json:"draft" yaml:"draft"`
	Conflicting  bool   `json:"conflicting" yaml:"conflicting"`
	ChecksTotal  int    `json:"checks_total" yaml:"checks_total"`
	ChecksPassed int    `json:"checks_passed" yaml:"checks_passed"`
}

// Briefing is what a reviewer reads about one finished agent turn. BuiltAt is
// Unix seconds.
type Briefing struct {
	PromptID string        `json:"prompt_id" yaml:"prompt_id"`
	BuiltAt  int64         `json:"built_at" yaml:"built_at"`
	Prompt   string        `json:"prompt" yaml:"prompt"`
	Closing  string        `json:"closing" yaml:"closing"`
	Checks   []CheckResult `json:"checks" yaml:"checks"`
	Git      *GitFacts     `json:"git,omitempty" yaml:"git,omitempty"`
	PR       *PRFacts      `json:"pr,omitempty" yaml:"pr,omitempty"`
	Warnings []string      `json:"warnings" yaml:"warnings"`
	Notes    []string      `json:"notes" yaml:"notes"`
}

// Warnings are things the reader should look at first; notes explain a gap in
// the data.
const (
	WarnCIRed       = "ci_red"
	WarnPRBlocked   = "pr_blocked"
	WarnCheckFailed = "check_failed"
	WarnEndedError  = "ended_error"
	WarnDirty       = "dirty"
	WarnBigDiff     = "big_diff"

	NoteDiffSlow = "diff_slow"
	NoteCIPaused = "ci_paused"
	NoteCIStale  = "ci_stale"
)

// warningsFor derives the warnings of a briefing, most urgent first.
func warningsFor(b Briefing, endedError, bigDiff bool) []string {
	var w []string
	if b.PR != nil && b.PR.Checks == "red" {
		w = append(w, WarnCIRed)
	}
	if b.PR != nil && (b.PR.Conflicting || b.PR.Review == "CHANGES_REQUESTED") {
		w = append(w, WarnPRBlocked)
	}
	for _, c := range b.Checks {
		if c.Result == CheckFail {
			w = append(w, WarnCheckFailed)
			break
		}
	}
	if endedError {
		w = append(w, WarnEndedError)
	}
	if b.Git != nil && b.Git.Uncommitted > 0 {
		w = append(w, WarnDirty)
	}
	if bigDiff {
		w = append(w, WarnBigDiff)
	}
	return w
}

const closingMax = 600

// closingWords is the last paragraph of the agent's final message, on one
// line and at most closingMax runes.
func closingWords(msg string) string {
	paras := paragraphs(msg)
	if len(paras) == 0 {
		return ""
	}
	s := strings.Join(strings.Fields(paras[len(paras)-1]), " ")
	if r := []rune(s); len(r) > closingMax {
		return string(r[:closingMax])
	}
	return s
}
