package board

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrPlanTooLarge is returned when a plan exceeds the 50KB size limit.
var ErrPlanTooLarge = errors.New("plan exceeds 50KB limit")

const maxPlanBytes = 50 * 1024

// Board provides high-level CRUD operations for kanban tasks and plans,
// built on top of the git-ref RefStore.
type Board struct {
	store *RefStore
}

// NewBoard creates a Board backed by the git repository at repoDir.
func NewBoard(repoDir string) *Board {
	return &Board{store: NewRefStore(repoDir)}
}

// CreateTask persists a new task card. If ID is empty, one is generated.
// CreatedAt and UpdatedAt are set to the current time.
// Returns the card with generated ID and timestamps.
func (b *Board) CreateTask(card TaskCard) (TaskCard, error) {
	if card.ID == "" {
		id, err := generateID()
		if err != nil {
			return TaskCard{}, fmt.Errorf("generate task id: %w", err)
		}
		card.ID = id
	}
	now := time.Now().UTC().Format(time.RFC3339)
	card.CreatedAt = now
	card.UpdatedAt = now

	data, err := marshalCard(card)
	if err != nil {
		return TaskCard{}, fmt.Errorf("marshal task %s: %w", card.ID, err)
	}
	ref := taskContentRef(card.ID)
	if err := b.store.WriteRef(ref, data); err != nil {
		return TaskCard{}, err
	}
	return card, nil
}

// GetTask reads a task card by ID. Returns an error wrapping
// ErrRefNotFound if the task does not exist.
func (b *Board) GetTask(id string) (TaskCard, error) {
	ref := taskContentRef(id)
	data, err := b.store.ReadRef(ref)
	if err != nil {
		return TaskCard{}, err
	}
	return unmarshalCard(data)
}

// ListTasks returns all task cards on the board. Returns an empty slice
// when no tasks exist.
func (b *Board) ListTasks() ([]TaskCard, error) {
	refs, err := b.store.ListRefs("refs/mtui/tasks/")
	if err != nil {
		return nil, err
	}

	// Extract unique task IDs from refs like refs/mtui/tasks/<id>/content.
	idSet := make(map[string]struct{})
	for _, ref := range refs {
		id := extractTaskID(ref)
		if id != "" {
			idSet[id] = struct{}{}
		}
	}

	cards := make([]TaskCard, 0, len(idSet))
	for id := range idSet {
		card, err := b.GetTask(id)
		if err != nil {
			// Skip tasks whose content ref is missing (plan-only refs).
			if errors.Is(err, ErrRefNotFound) {
				continue
			}
			return nil, fmt.Errorf("read task %s: %w", id, err)
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// UpdateTask overwrites an existing task card. UpdatedAt is set to now.
func (b *Board) UpdateTask(card TaskCard) error {
	card.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := marshalCard(card)
	if err != nil {
		return fmt.Errorf("marshal task %s: %w", card.ID, err)
	}
	ref := taskContentRef(card.ID)
	return b.store.WriteRef(ref, data)
}

// DeleteTask removes a task and its associated plan (if any).
func (b *Board) DeleteTask(id string) error {
	contentRef := taskContentRef(id)
	if err := b.store.DeleteRef(contentRef); err != nil {
		return err
	}
	// Best-effort delete of the plan ref.
	planRef := taskPlanRef(id)
	exists, _ := b.store.RefExists(planRef)
	if exists {
		_ = b.store.DeleteRef(planRef)
	}
	return nil
}

// SavePlan writes an execution plan for a task. Returns ErrPlanTooLarge
// if the serialized plan exceeds 50KB.
func (b *Board) SavePlan(cardID string, plan Plan) error {
	plan.CardID = cardID
	data, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan for %s: %w", cardID, err)
	}
	if len(data) > maxPlanBytes {
		return ErrPlanTooLarge
	}
	ref := taskPlanRef(cardID)
	return b.store.WriteRef(ref, data)
}

// GetPlan reads the execution plan for a task. Returns an error wrapping
// ErrRefNotFound if no plan exists.
func (b *Board) GetPlan(cardID string) (Plan, error) {
	ref := taskPlanRef(cardID)
	data, err := b.store.ReadRef(ref)
	if err != nil {
		return Plan{}, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("unmarshal plan for %s: %w", cardID, err)
	}
	return plan, nil
}

// --- helpers ---

func taskContentRef(id string) string {
	return "refs/mtui/tasks/" + id + "/content"
}

func taskPlanRef(id string) string {
	return "refs/mtui/tasks/" + id + "/plan"
}

// extractTaskID pulls the task ID from a ref like refs/mtui/tasks/<id>/content.
func extractTaskID(ref string) string {
	const prefix = "refs/mtui/tasks/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	rest := ref[len(prefix):]
	idx := strings.Index(rest, "/")
	if idx <= 0 {
		return ""
	}
	return rest[:idx]
}

func generateID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// marshalCard serializes a TaskCard to YAML frontmatter + markdown body.
func marshalCard(card TaskCard) ([]byte, error) {
	desc := card.Description
	yamlBytes, err := yaml.Marshal(card)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	if desc != "" {
		buf.WriteString(desc)
		if !strings.HasSuffix(desc, "\n") {
			buf.WriteString("\n")
		}
	}
	return []byte(buf.String()), nil
}

// unmarshalCard deserializes a TaskCard from YAML frontmatter + markdown body.
func unmarshalCard(data []byte) (TaskCard, error) {
	text := string(data)

	// Expect format: ---\n<yaml>\n---\n<body>
	if !strings.HasPrefix(text, "---\n") {
		return TaskCard{}, fmt.Errorf("missing YAML frontmatter delimiter")
	}
	rest := text[4:] // skip first ---\n
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return TaskCard{}, fmt.Errorf("missing closing YAML frontmatter delimiter")
	}
	frontmatter := rest[:idx]
	body := rest[idx+5:] // skip \n---\n

	var card TaskCard
	if err := yaml.Unmarshal([]byte(frontmatter), &card); err != nil {
		return TaskCard{}, fmt.Errorf("unmarshal frontmatter: %w", err)
	}
	card.Description = strings.TrimRight(body, "\n")
	return card, nil
}
