package hub

import (
	"context"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/gitx"
)

// gitOut runs git through gitx and returns the trimmed stdout.
func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := gitx.CmdContext(ctx, dir, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func gitCount(ctx context.Context, dir string, args ...string) int {
	s, err := gitOut(ctx, dir, args...)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// gitBriefing measures the turn (startTree against a fresh snapshot of the
// work tree) and the branch against its target. It returns false outside a
// repository. It never fetches and never touches the real index. prBase may
// be nil.
func gitBriefing(ctx context.Context, dir, startTree, windowTarget string, prBase func(ghRepo, string) string, p BriefingPolicy) (gitResult, bool) {
	return measureGit(ctx, gitx.SnapshotTree, dir, startTree, windowTarget, prBase, p)
}

// snapshotFunc takes the work-tree snapshot; gitx.SnapshotTree outside tests.
type snapshotFunc func(ctx context.Context, dir string) (string, error)

// measureGit is gitBriefing with the stop snapshot taken by snap, so the
// briefing desk's test seam governs both ends of a turn.
func measureGit(ctx context.Context, snap snapshotFunc, dir, startTree, windowTarget string, prBase func(ghRepo, string) string, p BriefingPolicy) (gitResult, bool) {
	p = p.normalized()
	var res gitResult
	if _, err := gitOut(ctx, dir, "rev-parse", "--show-toplevel"); err != nil {
		return res, false
	}
	f := &res.Facts
	res.Head, _ = gitOut(ctx, dir, "rev-parse", "HEAD")
	if b, err := gitOut(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD"); err == nil && b != "" {
		f.Branch = b
	} else {
		f.Detached = true
	}

	remote, hasUpstream := upstreamRemote(ctx, dir, f.Branch)
	f.NoUpstream = !hasUpstream
	if remote == "" {
		remote = "origin"
	}
	if url, err := gitOut(ctx, dir, "remote", "get-url", remote); err == nil {
		res.Repo, _ = parseGitHubRemote(url)
	}
	if hasUpstream {
		f.Unpushed = gitCount(ctx, dir, "rev-list", "--count", "@{upstream}..HEAD")
	}
	if s, err := gitOut(ctx, dir, "status", "--porcelain"); err == nil && s != "" {
		f.Uncommitted = len(strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n"))
	}

	var base string
	if prBase != nil {
		base = prBase(res.Repo, f.Branch)
	}
	f.Target = resolveTarget(ctx, dir, base, windowTarget)
	f.OnTarget = !f.Detached && f.Branch == f.Target

	stop, err := snap(ctx, dir)
	if err != nil {
		return res, true
	}
	if startTree != "" {
		if st, err := gitx.NumStat(ctx, dir, startTree, stop); err == nil {
			f.TurnKnown = true
			var lines int
			f.TurnFiles, f.TurnAdded, f.TurnDeleted, lines = sumStats(st)
			if f.OnTarget {
				res.BigDiff = f.TurnFiles > p.BigDiffFiles || lines > p.BigDiffLines
			}
		}
	}
	if f.OnTarget {
		return res, true
	}
	cmp := compareRef(ctx, dir, f.Target)
	f.Commits = gitCount(ctx, dir, "rev-list", "--count", cmp+"..HEAD")
	if mb, err := gitOut(ctx, dir, "merge-base", cmp, "HEAD"); err == nil && mb != "" {
		if st, err := gitx.NumStat(ctx, dir, mb, stop); err == nil {
			var lines int
			f.BranchFiles, f.BranchAdded, f.BranchDeleted, lines = sumStats(st)
			res.BigDiff = f.BranchFiles > p.BigDiffFiles || lines > p.BigDiffLines
		}
	}
	return res, true
}

// sumStats returns the distinct file count, added and deleted lines, and the
// changed lines without lockfiles (what the big-diff rule counts).
func sumStats(st []gitx.FileStat) (files, added, deleted, linesNoLock int) {
	seen := map[string]bool{}
	for _, s := range st {
		if !seen[s.Path] {
			seen[s.Path] = true
			files++
		}
		added += s.Added
		deleted += s.Deleted
		if !isLockfile(s.Path) {
			linesNoLock += s.Added + s.Deleted
		}
	}
	return
}

// upstreamRemote returns the remote of the branch's upstream, if it has one.
func upstreamRemote(ctx context.Context, dir, branch string) (string, bool) {
	if branch == "" {
		return "", false
	}
	if _, err := gitOut(ctx, dir, "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		return "", false
	}
	r, _ := gitOut(ctx, dir, "config", "branch."+branch+".remote")
	return r, true
}

// resolveTarget picks the target branch: the PR base, else the window's
// choice, else origin/HEAD, else "main". It returns the short branch name.
func resolveTarget(ctx context.Context, dir, prBase, windowTarget string) string {
	if prBase != "" {
		return prBase
	}
	if windowTarget != "" {
		return windowTarget
	}
	if s, err := gitOut(ctx, dir, "symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD"); err == nil && s != "" {
		return strings.TrimPrefix(s, "origin/")
	}
	return "main"
}

// compareRef is origin/<target> when that ref exists, else <target>.
func compareRef(ctx context.Context, dir, target string) string {
	if _, err := gitOut(ctx, dir, "rev-parse", "--verify", "-q", "refs/remotes/origin/"+target); err == nil {
		return "origin/" + target
	}
	return target
}

var ghRemoteRe = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^/\s]+)/([^/\s]+?)(?:\.git)?/?$`)

// parseGitHubRemote reads owner and name from a GitHub remote URL.
func parseGitHubRemote(url string) (ghRepo, bool) {
	m := ghRemoteRe.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return ghRepo{}, false
	}
	return ghRepo{Owner: m[1], Name: m[2]}, true
}

// isLockfile reports whether p is a dependency lockfile, whose lines do not
// count towards a big diff.
func isLockfile(p string) bool {
	switch path.Base(strings.ReplaceAll(p, "\\", "/")) {
	case "package-lock.json", "go.sum", "pnpm-lock.yaml", "yarn.lock", "Cargo.lock", "composer.lock":
		return true
	}
	return false
}
