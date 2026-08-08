package backend

import "strings"

// GetRepoURL returns the GitHub web URL for dir's "origin" remote (e.g.
// "https://github.com/owner/repo"), or "" if there is no git repo, no origin
// remote, or the remote isn't a github.com URL. Used by the footer to link
// directly to the repo's Issues/Pull Requests pages.
func (a *AppService) GetRepoURL(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := gitCmd(dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return normalizeGitHubURL(strings.TrimSpace(string(out)))
}

// normalizeGitHubURL converts an SSH or HTTPS github.com remote URL into the
// plain HTTPS web URL (no ".git" suffix). Returns "" for non-GitHub remotes.
func normalizeGitHubURL(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		return "https://github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		return "https://github.com/" + strings.TrimPrefix(remote, "ssh://git@github.com/")
	case strings.HasPrefix(remote, "https://github.com/"):
		return remote
	default:
		return ""
	}
}
