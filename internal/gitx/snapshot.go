package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/procs"
)

// CmdContext is Cmd with a context: the same flags, environment and console
// handling, and the process is killed when ctx ends.
func CmdContext(ctx context.Context, dir string, args ...string) *exec.Cmd {
	full := append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	procs.HideConsole(cmd)
	return cmd
}

// SnapshotTree writes the whole work tree (tracked, modified and new files,
// minus what .gitignore excludes) into a git tree and returns its id.
//
// The real index and work tree are never touched: the agent or the user may
// be running git at the same moment. The snapshot works on a temporary copy
// of the index, selected with GIT_INDEX_FILE. The copy lives outside the
// repository and is removed on every path. A repo without an index yet (fresh
// git init) starts from an empty one.
func SnapshotTree(ctx context.Context, dir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	out, err := CmdContext(ctx, dir, "rev-parse", "--git-path", "index").Output()
	if err != nil {
		return "", fmt.Errorf("locate index: %w", err)
	}
	src := strings.TrimSpace(string(out))
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, filepath.FromSlash(src))
	}

	tmp, err := os.CreateTemp("", "mtui-index-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() {
		_ = os.Remove(tmpPath)
		_ = os.Remove(tmpPath + ".lock")
	}()

	data, err := os.ReadFile(src)
	switch {
	case err == nil:
		if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
			return "", err
		}
	case os.IsNotExist(err):
		// An empty file is not a valid index; git creates one at a missing path.
		if err := os.Remove(tmpPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("read index: %w", err)
	}

	env := "GIT_INDEX_FILE=" + tmpPath
	add := CmdContext(ctx, dir, "add", "-A")
	add.Env = append(add.Env, env)
	if b, err := add.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %w: %s", err, strings.TrimSpace(string(b)))
	}
	wt := CmdContext(ctx, dir, "write-tree")
	wt.Env = append(wt.Env, env)
	b, err := wt.Output()
	if err != nil {
		return "", fmt.Errorf("git write-tree: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// FileStat is one file of a numstat: lines added and deleted. Binary files
// have no line counts.
type FileStat struct {
	Path    string
	Added   int
	Deleted int
	Binary  bool
}

// NumStat returns the per-file line changes between two trees, ordered by
// path. Renames are reported as a delete and an add (--no-renames), so every
// record has exactly one path.
func NumStat(ctx context.Context, dir, from, to string) ([]FileStat, error) {
	out, err := CmdContext(ctx, dir, "diff", "--numstat", "--no-renames", "-z", from, to).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	var stats []FileStat
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		parts := strings.SplitN(string(rec), "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected numstat record %q", rec)
		}
		fs := FileStat{Path: parts[2]}
		if parts[0] == "-" && parts[1] == "-" {
			fs.Binary = true
		} else {
			if fs.Added, err = strconv.Atoi(parts[0]); err != nil {
				return nil, fmt.Errorf("numstat added %q: %w", parts[0], err)
			}
			if fs.Deleted, err = strconv.Atoi(parts[1]); err != nil {
				return nil, fmt.Errorf("numstat deleted %q: %w", parts[1], err)
			}
		}
		stats = append(stats, fs)
	}
	return stats, nil
}
