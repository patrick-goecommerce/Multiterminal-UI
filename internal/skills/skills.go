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

// allDefs returns metadata for all 28 skills.
func allDefs() []skillDef {
	return []skillDef{
		{"frontend-react", "React/Next.js Specialist", "React, Next.js, hooks, SSR/SSG patterns", "frontend", []string{"package.json:react"}},
		{"frontend-vue", "Vue/Nuxt Specialist", "Vue 3, Nuxt, Composition API", "frontend", []string{"package.json:vue"}},
		{"frontend-svelte", "Svelte/SvelteKit Specialist", "Svelte 4/5, SvelteKit, stores", "frontend", []string{"package.json:svelte"}},
		{"frontend-angular", "Angular Specialist", "Angular, RxJS, NgModules", "frontend", []string{"angular.json"}},
		{"frontend-css", "CSS/Tailwind Specialist", "CSS architecture, Tailwind, responsive design", "frontend", []string{"tailwind.config.*"}},
		{"backend-go", "Go Backend Specialist", "Go idioms, concurrency, stdlib patterns", "backend", []string{"go.mod"}},
		{"backend-node", "Node.js/Express Specialist", "Node.js, Express, middleware patterns", "backend", []string{"package.json:express"}},
		{"backend-python", "Python/FastAPI Specialist", "Python, FastAPI, Django, type hints", "backend", []string{"requirements.txt", "pyproject.toml"}},
		{"backend-rust", "Rust Specialist", "Rust, ownership, async, crates", "backend", []string{"Cargo.toml"}},
		{"backend-java", "Java/Spring Specialist", "Java, Spring Boot, Maven/Gradle", "backend", []string{"pom.xml", "build.gradle"}},
		{"backend-csharp", "C#/.NET Specialist", "C#, .NET, ASP.NET Core, Entity Framework", "backend", []string{"*.csproj", "*.sln"}},
		{"backend-ruby", "Ruby/Rails Specialist", "Ruby, Rails, ActiveRecord, gems", "backend", []string{"Gemfile"}},
		{"backend-php", "PHP/Laravel Specialist", "PHP 8, Laravel, Composer", "backend", []string{"composer.json"}},
		{"api-design", "API Design Specialist", "REST, GraphQL, OpenAPI, versioning", "backend", []string{"openapi.*", "swagger.*"}},
		{"database-sql", "SQL/Postgres Specialist", "PostgreSQL, SQL optimization, migrations", "data", []string{"*.sql", "prisma/", "migrations/"}},
		{"database-nosql", "NoSQL/MongoDB Specialist", "MongoDB, Redis, document modeling", "data", []string{"package.json:mongoose"}},
		{"devops-docker", "Docker/Container Specialist", "Dockerfile optimization, multi-stage builds", "devops", []string{"Dockerfile", "docker-compose.*"}},
		{"devops-k8s", "Kubernetes Specialist", "K8s manifests, Helm, operators", "devops", []string{"k8s/", "helm/"}},
		{"devops-ci", "CI/CD Specialist", "GitHub Actions, GitLab CI, pipelines", "devops", []string{".github/workflows/", ".gitlab-ci.yml"}},
		{"devops-terraform", "Terraform/IaC Specialist", "Terraform, modules, state management", "devops", []string{"*.tf", "terraform/"}},
		{"devops-aws", "AWS Specialist", "AWS services, CDK, serverless", "devops", []string{"cdk.json", "serverless.yml"}},
		{"security", "Security Specialist", "OWASP, auth, encryption, vulnerability assessment", "quality", nil},
		{"testing", "Testing Specialist", "Unit, integration, e2e testing strategies", "quality", []string{"*_test.go", "*.test.ts", "*.spec.ts"}},
		{"performance", "Performance Specialist", "Profiling, optimization, caching, load testing", "quality", nil},
		{"accessibility", "Accessibility Specialist", "WCAG, ARIA, screen readers, a11y testing", "quality", []string{"*.html", "*.jsx", "*.tsx"}},
		{"mobile-rn", "React Native Specialist", "React Native, Expo, mobile patterns", "frontend", []string{"package.json:react-native"}},
		{"mobile-flutter", "Flutter/Dart Specialist", "Flutter, Dart, widgets, state management", "frontend", []string{"pubspec.yaml"}},
		{"docs-technical", "Technical Writing Specialist", "Documentation, ADRs, API docs, READMEs", "quality", []string{"docs/"}},
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
