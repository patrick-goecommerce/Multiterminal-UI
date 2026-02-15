// Package backend provides JSON parsing helpers for GitHub Issues data.
package backend

import (
	"encoding/json"
	"log"
)

// ghIssueRaw is the raw JSON structure returned by gh issue list/view.
type ghIssueRaw struct {
	Number    int             `json:"number"`
	Title     string          `json:"title"`
	State     string          `json:"state"`
	Author    ghAuthor        `json:"author"`
	Labels    []ghLabel       `json:"labels"`
	Body      string          `json:"body"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	Assignees []ghAuthor      `json:"assignees"`
	Comments  json.RawMessage `json:"comments"`
	URL       string          `json:"url"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghCommentRaw struct {
	Author    ghAuthor `json:"author"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"createdAt"`
}

// parseIssueList parses the JSON output of gh issue list.
func parseIssueList(data []byte) []Issue {
	var raw []ghIssueRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("[parseIssueList] parse error: %v", err)
		return nil
	}

	issues := make([]Issue, 0, len(raw))
	for _, r := range raw {
		labels := make([]string, 0, len(r.Labels))
		for _, l := range r.Labels {
			labels = append(labels, l.Name)
		}
		commentCount := parseCommentCount(r.Comments)
		issues = append(issues, Issue{
			Number:    r.Number,
			Title:     r.Title,
			State:     r.State,
			Author:    r.Author.Login,
			Labels:    labels,
			Body:      r.Body,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			Comments:  commentCount,
			URL:       r.URL,
		})
	}
	return issues
}

// parseIssueDetail parses the JSON output of gh issue view.
func parseIssueDetail(data []byte) *IssueDetail {
	var r ghIssueRaw
	if err := json.Unmarshal(data, &r); err != nil {
		log.Printf("[parseIssueDetail] parse error: %v", err)
		return nil
	}

	labels := make([]string, 0, len(r.Labels))
	for _, l := range r.Labels {
		labels = append(labels, l.Name)
	}

	assignees := make([]string, 0, len(r.Assignees))
	for _, a := range r.Assignees {
		assignees = append(assignees, a.Login)
	}

	comments := parseComments(r.Comments)

	return &IssueDetail{
		Number:    r.Number,
		Title:     r.Title,
		State:     r.State,
		Author:    r.Author.Login,
		Labels:    labels,
		Body:      r.Body,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		Assignees: assignees,
		URL:       r.URL,
		Comments:  comments,
	}
}

// parseCommentCount extracts comment count from the raw JSON comments field.
// gh returns comments as an array of objects or as a number depending on context.
func parseCommentCount(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	// Try as array first (gh issue list returns array)
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return len(arr)
	}
	// Try as number
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	return 0
}

// parseComments extracts comment details from the raw JSON comments field.
func parseComments(raw json.RawMessage) []IssueComment {
	if len(raw) == 0 {
		return nil
	}
	var rawComments []ghCommentRaw
	if err := json.Unmarshal(raw, &rawComments); err != nil {
		return nil
	}
	comments := make([]IssueComment, 0, len(rawComments))
	for _, c := range rawComments {
		comments = append(comments, IssueComment{
			Author:    c.Author.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		})
	}
	return comments
}
