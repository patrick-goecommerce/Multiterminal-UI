package orchestrator

// StepStatus represents the outcome of a step execution.
type StepStatus string

const (
	StepSuccess        StepStatus = "success"
	StepFailed         StepStatus = "failed"
	StepTimeout        StepStatus = "timeout"
	StepBudgetExceeded StepStatus = "budget_exceeded"
	StepStuck          StepStatus = "stuck"
)

// VerifyStep defines a verification command to run after implementation.
type VerifyStep struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ExecutionRequest is what the Orchestrator sends to the Engine.
type ExecutionRequest struct {
	StepID       string       `json:"step_id"`
	CardID       string       `json:"card_id"`
	WorktreeSlot int          `json:"worktree_slot"`
	Prompt       string       `json:"prompt"`
	SystemPrompt string       `json:"system_prompt"`
	Model        string       `json:"model"`
	Verify       []VerifyStep `json:"verify"`
	BudgetUSD    float64      `json:"budget_usd"`
	TimeoutSec   int          `json:"timeout_sec"`
	SkillPrompts []string     `json:"skill_prompts"`
}

// VerifyResult is the outcome of running a verification command.
type VerifyResult struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	ExitCode    int    `json:"exit_code"`
	Output      string `json:"output"`
	Passed      bool   `json:"passed"`
}

// LoopSignal indicates a detected pathological pattern.
type LoopSignal struct {
	Type   string `json:"type"`   // "same_error" | "fix_chain" | "file_churn" etc.
	Detail string `json:"detail"`
	Source string `json:"source"` // "step" | "repo"
}

// StepError describes why a step failed.
type StepError struct {
	Class   string `json:"class"`   // "build" | "test" | "timeout" | "budget" | "crash" | "scope_exceeded"
	Message string `json:"message"`
}

// ExecutionResult is what the Engine returns to the Orchestrator.
type ExecutionResult struct {
	StepID       string         `json:"step_id"`
	Status       StepStatus     `json:"status"`
	FilesChanged []string       `json:"files_changed"`
	FilesCreated []string       `json:"files_created"`
	Verify       []VerifyResult `json:"verify"`
	LoopSignals  []LoopSignal   `json:"loop_signals"`
	CostUSD      float64        `json:"cost_usd"`
	DurationSec  int            `json:"duration_sec"`
	Error        *StepError     `json:"error,omitempty"`
}

// MustHaves defines the required outcomes for a plan step.
type MustHaves struct {
	Truths    []string              `json:"truths"`
	Artifacts []ArtifactRequirement `json:"artifacts"`
}

// ArtifactRequirement defines a file that must exist after a step.
type ArtifactRequirement struct {
	Path     string `json:"path"`
	MinLines int    `json:"min_lines,omitempty"`
}

// PlanStep defines a single step in an execution plan.
type PlanStep struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Wave        int          `json:"wave"`
	DependsOn   []string     `json:"depends_on"`
	ParallelOk  bool         `json:"parallel_ok"`
	Model       string       `json:"model"`
	FilesModify []string     `json:"files_modify"`
	FilesCreate []string     `json:"files_create"`
	MustHaves   MustHaves    `json:"must_haves"`
	Verify      []VerifyStep `json:"verify"`
	Status      string       `json:"status"` // "pending" | "running" | "done" | "failed" | "stuck"
}

// Plan defines the full execution plan for a card.
type Plan struct {
	CardID     string     `json:"card_id"`
	Complexity string     `json:"complexity"`
	Steps      []PlanStep `json:"steps"`
}
