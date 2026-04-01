package board

// TaskState represents the current state of a task in the state machine.
type TaskState string

const (
	StateBacklog     TaskState = "backlog"
	StateTriage      TaskState = "triage"
	StatePlanning    TaskState = "planning"
	StateReview      TaskState = "review"
	StateExecuting   TaskState = "executing"
	StateStuck       TaskState = "stuck"
	StateQA          TaskState = "qa"
	StateMerging     TaskState = "merging"
	StateHumanReview TaskState = "human_review"
	StateDone        TaskState = "done"
)

// CardType categorizes a task for scope limits and routing.
type CardType string

const (
	CardTypeBugfix   CardType = "bugfix"
	CardTypeFeature  CardType = "feature"
	CardTypeRefactor CardType = "refactor"
	CardTypeDocs     CardType = "docs"
)

// Complexity from triage assessment.
type Complexity string

const (
	ComplexityTrivial Complexity = "trivial"
	ComplexityMedium  Complexity = "medium"
	ComplexityComplex Complexity = "complex"
)

// TaskCard is the main board entity stored in refs/mtui/tasks/<id>/content.
type TaskCard struct {
	ID          string     `yaml:"id" json:"id"`
	Title       string     `yaml:"title" json:"title"`
	Description string     `yaml:"-" json:"description"`
	State       TaskState  `yaml:"state" json:"state"`
	CardType    CardType   `yaml:"card_type" json:"card_type"`
	Complexity  Complexity `yaml:"complexity" json:"complexity"`

	// Metadata
	CreatedAt string `yaml:"created_at" json:"created_at"`
	UpdatedAt string `yaml:"updated_at" json:"updated_at"`

	// Execution tracking
	ExecutionMode string  `yaml:"execution_mode,omitempty" json:"execution_mode,omitempty"`
	ReviewReason  string  `yaml:"review_reason,omitempty" json:"review_reason,omitempty"`
	QAAttempts    int     `yaml:"qa_attempts" json:"qa_attempts"`
	EscAttempts   int     `yaml:"esc_attempts" json:"esc_attempts"`
	CostUSD       float64 `yaml:"cost_usd" json:"cost_usd"`
}

// PlanStep represents one step in an execution plan.
type PlanStep struct {
	ID          string   `yaml:"id" json:"id"`
	Title       string   `yaml:"title" json:"title"`
	Wave        int      `yaml:"wave" json:"wave"`
	DependsOn   []string `yaml:"depends_on" json:"depends_on"`
	ParallelOk  bool     `yaml:"parallel_ok" json:"parallel_ok"`
	Model       string   `yaml:"model" json:"model"`
	FilesModify []string `yaml:"files_modify" json:"files_modify"`
	FilesCreate []string `yaml:"files_create" json:"files_create"`
	Status      string   `yaml:"status" json:"status"`
}

// Plan is the execution plan stored in refs/mtui/tasks/<id>/plan.
type Plan struct {
	CardID     string     `yaml:"card_id" json:"card_id"`
	Complexity Complexity `yaml:"complexity" json:"complexity"`
	Steps      []PlanStep `yaml:"steps" json:"steps"`
}
