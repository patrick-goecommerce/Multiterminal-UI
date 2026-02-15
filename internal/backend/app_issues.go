// Package backend provides GitHub Issues integration via the gh CLI.
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

// Issue represents a GitHub issue summary for list views.
type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	Comments  int      `json:"comments"`
	URL       string   `json:"url"`
}

// IssueDetail represents a full GitHub issue with body and comments.
type IssueDetail struct {
	Number    int            `json:"number"`
	Title     string         `json:"title"`
	State     string         `json:"state"`
	Author    string         `json:"author"`
	Labels    []string       `json:"labels"`
	Body      string         `json:"body"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
	Assignees []string       `json:"assignees"`
	URL       string         `json:"url"`
	Comments  []IssueComment `json:"comments"`
}

// IssueComment represents a single comment on a GitHub issue.
type IssueComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// IssueLabel represents a GitHub label with name and color.
type IssueLabel struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// CheckGitHubCLI checks if the gh CLI is installed and authenticated.
// Returns "ok", "not_installed", or "not_authenticated".
func (a *App) CheckGitHubCLI() string {
	if _, err := exec.LookPath("gh"); err != nil {
		return "not_installed"
	}
	cmd := exec.Command("gh", "auth", "status")
	hideConsole(cmd)
	if err := cmd.Run(); err != nil {
		return "not_authenticated"
	}
	return "ok"
}

// GetIssues returns a list of GitHub issues for the repo in dir.
// state can be "open", "closed", or "all".
func (a *App) GetIssues(dir string, state string) []Issue {
	if dir == "" {
		return nil
	}
	if state == "" {
		state = "open"
	}

	fields := "number,title,state,author,labels,createdAt,updatedAt,comments,url"
	cmd := exec.Command("gh", "issue", "list",
		"--state", state,
		"--limit", "50",
		"--json", fields,
	)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[GetIssues] gh error: %v", err)
		return nil
	}

	return parseIssueList(out)
}

// GetIssueDetail returns the full details of a single issue including comments.
func (a *App) GetIssueDetail(dir string, number int) *IssueDetail {
	if dir == "" || number <= 0 {
		return nil
	}

	fields := "number,title,state,author,labels,body,createdAt,updatedAt,assignees,comments,url"
	cmd := exec.Command("gh", "issue", "view",
		strconv.Itoa(number),
		"--json", fields,
	)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[GetIssueDetail] gh error: %v", err)
		return nil
	}

	return parseIssueDetail(out)
}

// CreateIssue creates a new GitHub issue and returns it.
func (a *App) CreateIssue(dir string, title string, body string, labels []string) *Issue {
	if dir == "" || title == "" {
		return nil
	}

	args := []string{"issue", "create", "--title", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	for _, label := range labels {
		if label != "" {
			args = append(args, "--label", label)
		}
	}

	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[CreateIssue] gh error: %v", err)
		return nil
	}

	// gh issue create outputs the URL of the created issue
	url := strings.TrimSpace(string(out))
	// Extract issue number from URL (last path segment)
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return nil
	}
	num, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return nil
	}

	return &Issue{
		Number: num,
		Title:  title,
		State:  "open",
		Labels: labels,
		URL:    url,
	}
}

// UpdateIssue updates an existing issue's title, body, and/or state.
func (a *App) UpdateIssue(dir string, number int, title string, body string, state string) error {
	if dir == "" || number <= 0 {
		return fmt.Errorf("invalid parameters")
	}

	numStr := strconv.Itoa(number)

	// Update title and body if provided
	if title != "" || body != "" {
		args := []string{"issue", "edit", numStr}
		if title != "" {
			args = append(args, "--title", title)
		}
		if body != "" {
			args = append(args, "--body", body)
		}
		cmd := exec.Command("gh", args...)
		cmd.Dir = dir
		hideConsole(cmd)
		if err := cmd.Run(); err != nil {
			log.Printf("[UpdateIssue] edit error: %v", err)
			return fmt.Errorf("issue edit failed: %w", err)
		}
	}

	// Update state if provided
	if state == "closed" {
		cmd := exec.Command("gh", "issue", "close", numStr)
		cmd.Dir = dir
		hideConsole(cmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("issue close failed: %w", err)
		}
	} else if state == "open" {
		cmd := exec.Command("gh", "issue", "reopen", numStr)
		cmd.Dir = dir
		hideConsole(cmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("issue reopen failed: %w", err)
		}
	}

	return nil
}

// AddIssueComment adds a comment to an existing issue.
func (a *App) AddIssueComment(dir string, number int, body string) error {
	if dir == "" || number <= 0 || body == "" {
		return fmt.Errorf("invalid parameters")
	}

	cmd := exec.Command("gh", "issue", "comment",
		strconv.Itoa(number),
		"--body", body,
	)
	cmd.Dir = dir
	hideConsole(cmd)
	if err := cmd.Run(); err != nil {
		log.Printf("[AddIssueComment] gh error: %v", err)
		return fmt.Errorf("comment failed: %w", err)
	}
	return nil
}

// GetIssueLabels returns all available labels for the repo in dir.
func (a *App) GetIssueLabels(dir string) []IssueLabel {
	if dir == "" {
		return nil
	}

	cmd := exec.Command("gh", "label", "list", "--json", "name,color", "--limit", "100")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[GetIssueLabels] gh error: %v", err)
		return nil
	}

	var labels []IssueLabel
	if err := json.Unmarshal(out, &labels); err != nil {
		log.Printf("[GetIssueLabels] parse error: %v", err)
		return nil
	}
	return labels
}
