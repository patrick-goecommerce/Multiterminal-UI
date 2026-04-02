package board

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ErrRefNotFound is returned when a git ref does not exist.
var ErrRefNotFound = errors.New("ref not found")

// validRefPattern restricts ref names to safe characters under refs/mtui/.
var validRefPattern = regexp.MustCompile(`^refs/mtui/[a-zA-Z0-9/_-]+$`)

// RefStore provides low-level git-ref read/write operations for storing
// arbitrary content as blobs pointed to by refs under refs/mtui/*.
type RefStore struct {
	repoDir string
}

// NewRefStore creates a RefStore rooted at the given git repository directory.
func NewRefStore(repoDir string) *RefStore {
	return &RefStore{repoDir: repoDir}
}

// ValidateGitRepo checks if the given directory is inside a git repository.
func ValidateGitRepo(dir string) error {
	if dir == "" {
		return fmt.Errorf("kein Projektverzeichnis ausgewählt")
	}
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'%s' ist kein Git-Repository — Kanban Board benötigt ein Git-Projekt", dir)
	}
	return nil
}

// WriteRef stores content as a blob in a git ref.
// The ref must be a valid path like "refs/mtui/tasks/abc123/content".
func (rs *RefStore) WriteRef(ref string, content []byte) error {
	if err := rs.validateRef(ref); err != nil {
		return err
	}
	ctx := context.Background()

	// Create blob from content.
	hashCmd := rs.git(ctx, "hash-object", "-w", "--stdin")
	hashCmd.Stdin = bytes.NewReader(content)
	var stderr bytes.Buffer
	hashCmd.Stderr = &stderr
	out, err := hashCmd.Output()
	if err != nil {
		return fmt.Errorf("git hash-object for ref %q (dir=%s): %w: %s", ref, rs.repoDir, err, strings.TrimSpace(stderr.String()))
	}
	sha := strings.TrimSpace(string(out))

	// Point the ref at the blob.
	if err := rs.gitRun(ctx, "update-ref", ref, sha); err != nil {
		return fmt.Errorf("git update-ref for ref %q: %w", ref, err)
	}
	return nil
}

// ReadRef reads content from a git ref. Returns ErrRefNotFound if the
// ref does not exist.
func (rs *RefStore) ReadRef(ref string) ([]byte, error) {
	if err := rs.validateRef(ref); err != nil {
		return nil, err
	}
	ctx := context.Background()

	out, err := rs.git(ctx, "show", ref).Output()
	if err != nil {
		// Distinguish "not found" from other failures.
		exists, _ := rs.RefExists(ref)
		if !exists {
			return nil, fmt.Errorf("read ref %q: %w", ref, ErrRefNotFound)
		}
		return nil, fmt.Errorf("git show for ref %q: %w", ref, err)
	}
	return out, nil
}

// DeleteRef removes a git ref. Returns ErrRefNotFound if the ref does
// not exist.
func (rs *RefStore) DeleteRef(ref string) error {
	if err := rs.validateRef(ref); err != nil {
		return err
	}
	ctx := context.Background()

	exists, err := rs.RefExists(ref)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("delete ref %q: %w", ref, ErrRefNotFound)
	}

	if err := rs.gitRun(ctx, "update-ref", "-d", ref); err != nil {
		return fmt.Errorf("git update-ref -d for ref %q: %w", ref, err)
	}
	return nil
}

// ListRefs returns all ref names matching the given prefix
// (e.g. "refs/mtui/tasks/"). Returns an empty slice when no refs match.
func (rs *RefStore) ListRefs(prefix string) ([]string, error) {
	if prefix != "" {
		if err := rs.validateRefPrefix(prefix); err != nil {
			return nil, err
		}
	}
	ctx := context.Background()

	out, err := rs.git(ctx, "for-each-ref", "--format=%(refname)", prefix).Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref with prefix %q: %w", prefix, err)
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}

// RefExists checks whether a ref exists.
func (rs *RefStore) RefExists(ref string) (bool, error) {
	if err := rs.validateRef(ref); err != nil {
		return false, err
	}
	ctx := context.Background()

	err := rs.gitRun(ctx, "show-ref", "--verify", "--quiet", ref)
	if err != nil {
		// Exit code 1 means "ref not found" — not a real error.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, fmt.Errorf("git show-ref for ref %q: %w", ref, err)
	}
	return true, nil
}

// validateRef ensures the ref matches the allowed pattern.
func (rs *RefStore) validateRef(ref string) error {
	if !validRefPattern.MatchString(ref) {
		return fmt.Errorf("invalid ref name %q: must match %s", ref, validRefPattern.String())
	}
	return nil
}

// validRefPrefixPattern allows prefixes that are partial ref paths under refs/mtui/.
var validRefPrefixPattern = regexp.MustCompile(`^refs/mtui/[a-zA-Z0-9/_-]*$`)

// validateRefPrefix validates a prefix used for listing refs.
func (rs *RefStore) validateRefPrefix(prefix string) error {
	if !validRefPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("invalid ref prefix %q: must match %s", prefix, validRefPrefixPattern.String())
	}
	return nil
}

// git creates an exec.Cmd for the given git subcommand and arguments,
// with the working directory set to the repository root.
func (rs *RefStore) git(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = rs.repoDir
	return cmd
}

// gitRun executes a git command and returns any error.
func (rs *RefStore) gitRun(ctx context.Context, args ...string) error {
	cmd := rs.git(ctx, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}
