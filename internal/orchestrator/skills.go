package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a loadable skill with detection rules and policies.
type Skill struct {
	Name       string      `json:"name"`
	Detect     []string    `json:"detect"`      // file patterns to detect (e.g. "go.mod")
	Priority   int         `json:"priority"`
	Stackable  bool        `json:"stackable"`
	PromptFile string      `json:"prompt_file"`
	Policies   SkillPolicy `json:"policies"`
}

// SkillPolicy defines the policies a skill contributes.
type SkillPolicy struct {
	PreferredModel string               `json:"preferred_model,omitempty"`
	Verify         []VerifyStep         `json:"verify,omitempty"`
	ScopeLimits    map[string]ScopeLimit `json:"scope_limits,omitempty"`
	QARules        []string             `json:"qa_rules,omitempty"`
}

// ScopeLimit defines maximum change thresholds per card type.
type ScopeLimit struct {
	MaxFiles   int `json:"max_files"`
	MaxLines   int `json:"max_lines"`
	MaxDeletes int `json:"max_deletes"`
}

// MergedPolicy is the result of merging multiple skills' policies.
type MergedPolicy struct {
	PreferredModel string
	Verify         []VerifyStep
	ScopeLimits    map[string]ScopeLimit
	QARules        []string
	SkillPrompts   []string // loaded prompt file contents
}

// knownFiles lists filenames that DetectStack looks for.
var knownFiles = []string{
	"go.mod",
	"package.json",
	"Cargo.toml",
	"requirements.txt",
	"pyproject.toml",
	"composer.json",
	"Gemfile",
	"pom.xml",
	"build.gradle",
	"tsconfig.json",
	"svelte.config.js",
	"svelte.config.ts",
	"next.config.js",
	"next.config.ts",
	"next.config.mjs",
	"angular.json",
	"vue.config.js",
	"nuxt.config.ts",
	"Dockerfile",
	".claude",
}

// DetectStack scans a directory for known project files and returns matches.
func DetectStack(dir string) []string {
	var found []string
	for _, name := range knownFiles {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	return found
}

// LoadSkills reads all .json skill manifests from a directory.
func LoadSkills(skillDir string) ([]Skill, error) {
	matches, err := filepath.Glob(filepath.Join(skillDir, "*.json"))
	if err != nil {
		return nil, err
	}
	var skills []Skill
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var s Skill
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	// Sort by priority descending (highest first).
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Priority > skills[j].Priority
	})
	return skills, nil
}

// MatchSkills filters skills based on detected stack files.
// A skill matches if ANY of its detect patterns appear in detected,
// or if its detect list is empty (universal skill).
func MatchSkills(detected []string, allSkills []Skill) []Skill {
	detectedSet := make(map[string]bool, len(detected))
	for _, d := range detected {
		detectedSet[d] = true
	}

	var matched []Skill
	for _, sk := range allSkills {
		if len(sk.Detect) == 0 {
			matched = append(matched, sk)
			continue
		}
		for _, pat := range sk.Detect {
			if detectedSet[pat] {
				matched = append(matched, sk)
				break
			}
		}
	}
	return matched
}

// modelRank returns a numeric rank for model preference merging.
// Higher rank wins during merge.
func modelRank(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return 3
	case strings.Contains(m, "sonnet"):
		return 2
	case strings.Contains(m, "haiku"):
		return 1
	default:
		return 0
	}
}

// MergePolicies combines policies from multiple skills.
func MergePolicies(skills []Skill, skillDir string) MergedPolicy {
	mp := MergedPolicy{
		ScopeLimits: make(map[string]ScopeLimit),
	}

	verifySeen := make(map[string]bool)
	qaSeen := make(map[string]bool)

	// Process skills in priority order (they should already be sorted).
	for _, sk := range skills {
		p := sk.Policies

		// preferred_model: highest model rank wins.
		if modelRank(p.PreferredModel) > modelRank(mp.PreferredModel) {
			mp.PreferredModel = p.PreferredModel
		}

		// verify: deduplicate by command string.
		for _, v := range p.Verify {
			if !verifySeen[v.Command] {
				verifySeen[v.Command] = true
				mp.Verify = append(mp.Verify, v)
			}
		}

		// scope_limits: strictest (minimum) wins per card type.
		for cardType, limit := range p.ScopeLimits {
			if existing, ok := mp.ScopeLimits[cardType]; ok {
				mp.ScopeLimits[cardType] = minScopeLimit(existing, limit)
			} else {
				mp.ScopeLimits[cardType] = limit
			}
		}

		// qa_rules: merge and deduplicate.
		for _, rule := range p.QARules {
			if !qaSeen[rule] {
				qaSeen[rule] = true
				mp.QARules = append(mp.QARules, rule)
			}
		}
	}

	// Load prompt files sorted by priority (skills are already sorted).
	for _, sk := range skills {
		if sk.PromptFile == "" {
			continue
		}
		path := filepath.Join(skillDir, sk.PromptFile)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			mp.SkillPrompts = append(mp.SkillPrompts, content)
		}
	}

	return mp
}

// minScopeLimit returns the element-wise minimum of two ScopeLimits.
func minScopeLimit(a, b ScopeLimit) ScopeLimit {
	return ScopeLimit{
		MaxFiles:   minInt(a.MaxFiles, b.MaxFiles),
		MaxLines:   minInt(a.MaxLines, b.MaxLines),
		MaxDeletes: minInt(a.MaxDeletes, b.MaxDeletes),
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
