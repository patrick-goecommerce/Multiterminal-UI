// Package skills provides specialist skill templates that can be auto-detected
// and injected into project CLAUDE.md files for better AI agent performance.
package skills

import (
	"embed"
	"path/filepath"
	"strings"
)

//go:embed templates/*.md
var skillFS embed.FS

// Skill represents a specialist persona template.
type Skill struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Category    string   `json:"category" yaml:"category"`
	DetectFiles []string `json:"detect_files" yaml:"detect_files"`
	Content     string   `json:"content" yaml:"content"`
}

// registry holds all known skills, populated at init time.
var registry []Skill

func init() {
	registry = buildRegistry()
}

// AllSkills returns all available skill templates.
func AllSkills() []Skill {
	return registry
}

// GetSkill returns a single skill by ID, or nil if not found.
func GetSkill(id string) *Skill {
	for i := range registry {
		if registry[i].ID == id {
			return &registry[i]
		}
	}
	return nil
}

// GetSkills returns skills matching the given IDs.
func GetSkills(ids []string) []Skill {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	var result []Skill
	for _, s := range registry {
		if set[s.ID] {
			result = append(result, s)
		}
	}
	return result
}

// skillDef defines a skill's metadata (content is loaded from embedded fs).
type skillDef struct {
	ID          string
	Name        string
	Description string
	Category    string
	DetectFiles []string
}

// LegacySkillMap maps old granular skill IDs to their consolidated replacements.
// Used for migrating existing projects to the new consolidated skill set.
var LegacySkillMap = map[string]string{
	"frontend-react":   "frontend",
	"frontend-vue":     "frontend",
	"frontend-svelte":  "frontend",
	"frontend-angular": "frontend",
	"frontend-css":     "frontend",
	"mobile-rn":        "frontend",
	"mobile-flutter":   "frontend",
	"backend-go":       "backend",
	"backend-node":     "backend",
	"backend-python":   "backend",
	"backend-rust":     "backend",
	"backend-java":     "backend",
	"backend-csharp":   "backend",
	"backend-ruby":     "backend",
	"backend-php":      "backend",
	"database-sql":     "database",
	"database-nosql":   "database",
	"devops-docker":    "devops",
	"devops-k8s":       "devops",
	"devops-ci":        "devops",
	"devops-terraform": "devops",
	"devops-aws":       "devops",
}

// MigrateLegacySkills converts old skill IDs to consolidated IDs, deduplicating.
func MigrateLegacySkills(ids []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, id := range ids {
		newID := id
		if mapped, ok := LegacySkillMap[id]; ok {
			newID = mapped
		}
		if !seen[newID] {
			seen[newID] = true
			result = append(result, newID)
		}
	}
	return result
}

// allDefs returns metadata for all 13 consolidated skills.
func allDefs() []skillDef {
	return []skillDef{
		// Development
		{"frontend", "Frontend Specialist", "React, Vue, Svelte, Angular, CSS, Mobile – framework-agnostisch", "frontend",
			[]string{"package.json:react", "package.json:vue", "package.json:svelte", "angular.json", "pubspec.yaml", "package.json:react-native"}},
		{"backend", "Backend Specialist", "Go, Node, Python, Rust, Java, C#, Ruby, PHP – sprachübergreifend", "backend",
			[]string{"go.mod", "package.json:express", "requirements.txt", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle", "*.csproj", "Gemfile", "composer.json"}},
		{"api-design", "API Design Specialist", "REST, GraphQL, OpenAPI, Versionierung", "backend",
			[]string{"openapi.*", "swagger.*"}},
		{"database", "Datenbank Specialist", "SQL, PostgreSQL, MongoDB, Redis, Migrationen", "data",
			[]string{"*.sql", "prisma/", "migrations/", "package.json:mongoose"}},

		// Operations
		{"devops", "DevOps Specialist", "Docker, Kubernetes, CI/CD, Terraform, AWS", "devops",
			[]string{"Dockerfile", "docker-compose.*", "k8s/", "helm/", ".github/workflows/", ".gitlab-ci.yml", "*.tf", "cdk.json", "serverless.yml"}},

		// Quality
		{"testing", "Testing Specialist", "Unit, Integration, E2E Testing Strategien", "quality",
			[]string{"*_test.go", "*.test.ts", "*.spec.ts"}},
		{"security", "Security Specialist", "OWASP, Auth, Verschlüsselung, Schwachstellenanalyse", "quality", nil},
		{"performance", "Performance Specialist", "Profiling, Optimierung, Caching, Lasttest", "quality", nil},
		{"accessibility", "Accessibility Specialist", "WCAG, ARIA, Screenreader, a11y Testing", "quality",
			[]string{"*.html", "*.jsx", "*.tsx"}},
		{"docs-technical", "Technical Writing Specialist", "Dokumentation, ADRs, API Docs, READMEs", "quality",
			[]string{"docs/"}},

		// Workflow
		{"git-workflow", "Git Workflow Specialist", "Branching, Conventional Commits, PRs, Code Review", "workflow",
			[]string{".git/"}},
		{"refactoring", "Refactoring Specialist", "Code-Restructuring, DRY, SOLID, Extract Method", "workflow", nil},
		{"code-review", "Code Review Specialist", "Review-Perspektive, PR-Feedback, Code Smells", "workflow", nil},
	}
}

// buildRegistry loads skill content from embedded templates.
func buildRegistry() []Skill {
	defs := allDefs()
	skills := make([]Skill, 0, len(defs))
	for _, d := range defs {
		filename := d.ID + ".md"
		content, err := skillFS.ReadFile(filepath.Join("templates", filename))
		if err != nil {
			// Template not found — use placeholder
			content = []byte("# " + d.Name + "\n\n" + d.Description + "\n")
		}
		skills = append(skills, Skill{
			ID:          d.ID,
			Name:        d.Name,
			Description: d.Description,
			Category:    d.Category,
			DetectFiles: d.DetectFiles,
			Content:     strings.TrimSpace(string(content)),
		})
	}
	return skills
}
