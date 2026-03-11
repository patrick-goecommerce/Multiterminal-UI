// Package backend provides the review pipeline for the agent orchestrator.
// When an agent finishes work on a Kanban card, this pipeline runs tests,
// spawns a review agent, creates a PR, and optionally auto-merges.
package backend

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// runReviewTests executes the review command in the card's worktree directory.
// Returns (passed, output). Uses COMSPEC on Windows per CLAUDE.md rules.
func (a *AppService) runReviewTests(card *KanbanCard, reviewCmd string) (bool, string) {
	if card.WorktreePath == "" || reviewCmd == "" {
		return true, ""
	}
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "sh"
	}
	var cmd *exec.Cmd
	if strings.Contains(shell, "cmd") {
		cmd = exec.Command(shell, "/c", reviewCmd)
	} else {
		cmd = exec.Command(shell, "-c", reviewCmd)
	}
	cmd.Dir = card.WorktreePath
	hideConsole(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[review] tests failed for card %s: %v", card.ID, err)
		return false, string(output)
	}
	return true, string(output)
}

// getWorktreeDiff returns the git diff HEAD output for a worktree directory.
func (a *AppService) getWorktreeDiff(worktreePath string) string {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = worktreePath
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[review] diff failed in %s: %v", worktreePath, err)
		return ""
	}
	return string(out)
}

// spawnReviewAgent creates a Claude session to review the diff from a worktree.
func (a *AppService) spawnReviewAgent(card *KanbanCard, dir string) int {
	diff := a.getWorktreeDiff(card.WorktreePath)
	if diff == "" {
		diff = "(no changes detected)"
	}
	const maxDiffLen = 50000
	if len(diff) > maxDiffLen {
		diff = diff[:maxDiffLen] + "\n... (truncated)"
	}
	prompt := fmt.Sprintf(
		"Review this diff for code quality, bugs, security issues, and adherence to project rules.\n"+
			"Respond with exactly REVIEW_PASS or REVIEW_FAIL on the first line, "+
			"followed by your findings.\n\nDiff:\n```\n%s\n```", diff)
	sessionID := a.CreateSession([]string{"claude"}, dir, 24, 80, "claude")
	if sessionID < 0 {
		log.Printf("[review] failed to create review session for card %s", card.ID)
		return -1
	}
	go func() {
		time.Sleep(2 * time.Second)
		a.AddToQueue(sessionID, prompt)
	}()
	log.Printf("[review] spawned review agent session %d for card %s", sessionID, card.ID)
	return sessionID
}

// createPRForCard commits changes, pushes the worktree branch, and creates a PR.
func (a *AppService) createPRForCard(card *KanbanCard, dir string) (int, error) {
	if card.WorktreePath == "" {
		return 0, fmt.Errorf("no worktree path for card %s", card.ID)
	}
	// Stage and commit all changes
	if err := runGit(card.WorktreePath, "add", "-A"); err != nil {
		return 0, fmt.Errorf("git add failed: %w", err)
	}
	commitMsg := fmt.Sprintf("feat: %s\n\nAgent-generated for card %s", card.Title, card.ID)
	commitCmd := exec.Command("git", "commit", "-m", commitMsg, "--allow-empty")
	commitCmd.Dir = card.WorktreePath
	hideConsole(commitCmd)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "nothing to commit") {
			return 0, fmt.Errorf("git commit failed: %s – %w", string(out), err)
		}
	}
	// Push the branch
	if err := runGit(card.WorktreePath, "push", "-u", "origin", card.WorktreeBranch); err != nil {
		return 0, fmt.Errorf("git push failed: %w", err)
	}
	// Create PR via gh CLI
	title := fmt.Sprintf("agent/%s: %s", card.ID, card.Title)
	body := fmt.Sprintf("Automated PR from agent orchestrator.\nCard: %s", card.ID)
	if card.ParentIssue > 0 {
		body += fmt.Sprintf("\nParent issue: #%d", card.ParentIssue)
	}
	prCmd := exec.Command("gh", "pr", "create",
		"--head", card.WorktreeBranch, "--title", title, "--body", body)
	prCmd.Dir = dir
	hideConsole(prCmd)
	prOut, err := prCmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pr create failed: %s – %w", string(prOut), err)
	}
	prNumber := parsePRNumberFromURL(strings.TrimSpace(string(prOut)))
	card.PRNumber = prNumber
	log.Printf("[review] created PR #%d for card %s", prNumber, card.ID)
	return prNumber, nil
}

// runGit runs a simple git command in dir. Returns error on failure.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", strings.Join(args, " "), string(out))
	}
	return nil
}

// parsePRNumberFromURL extracts the PR number from a GitHub URL.
func parsePRNumberFromURL(url string) int {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return 0
	}
	num, _ := strconv.Atoi(parts[len(parts)-1])
	return num
}

// autoMergeCard merges the PR via gh CLI and cleans up the worktree.
func (a *AppService) autoMergeCard(card *KanbanCard, dir string) error {
	if card.PRNumber <= 0 {
		return fmt.Errorf("no PR number for card %s", card.ID)
	}
	cmd := exec.Command("gh", "pr", "merge", strconv.Itoa(card.PRNumber),
		"--squash", "--delete-branch")
	cmd.Dir = dir
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge failed: %s – %w", string(out), err)
	}
	log.Printf("[review] auto-merged PR #%d for card %s", card.PRNumber, card.ID)
	return a.removeAgentWorktree(card)
}

// removeAgentWorktree removes the git worktree for a card and clears its path fields.
func (a *AppService) removeAgentWorktree(card *KanbanCard) error {
	if card.WorktreePath == "" {
		return nil
	}
	cmd := exec.Command("git", "worktree", "remove", "--force", card.WorktreePath)
	hideConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("worktree remove %s: %w", card.WorktreePath, err)
	}
	log.Printf("[review] removed worktree %s for card %s", card.WorktreePath, card.ID)
	card.WorktreePath = ""
	card.WorktreeBranch = ""
	return nil
}

// checkAndCloseParentIssue closes the parent issue if all sub-tickets are done.
func (a *AppService) checkAndCloseParentIssue(dir string, parentIssue int, state *KanbanState) {
	if parentIssue <= 0 {
		return
	}
	nonDoneCols := []string{ColDefine, ColRefine, ColApproved, ColReady, ColInProgress, ColAutoReview}
	for _, col := range nonDoneCols {
		for _, c := range state.Columns[col] {
			if c.ParentIssue == parentIssue {
				return
			}
		}
	}
	log.Printf("[review] all sub-tickets done for parent issue #%d, closing", parentIssue)
	_ = a.UpdateIssue(dir, parentIssue, "", "", "closed")
	_ = a.AddIssueComment(dir, parentIssue,
		"**Multiterminal Agent Orchestrator**\nAlle Sub-Tickets abgeschlossen. Issue wird geschlossen.")
	a.emitOrchestratorEvent("", "parent_closed")
}

// startReviewPipeline runs the full review pipeline for a completed agent card.
// Steps: tests -> review agent -> PR creation -> optional auto-merge -> done.
func (a *AppService) startReviewPipeline(card *KanbanCard, dir string, state *KanbanState, reviewCmd string) {
	log.Printf("[review] starting pipeline for card %s", card.ID)

	// Step 1: Run tests
	passed, testOutput := a.runReviewTests(card, reviewCmd)
	if !passed {
		if card.RetryCount < card.MaxRetries {
			card.RetryCount++
			card.ReviewResult = "test_fail"
			log.Printf("[review] tests failed for card %s, retry %d/%d", card.ID, card.RetryCount, card.MaxRetries)
			if card.AgentSessionID > 0 {
				a.AddToQueue(card.AgentSessionID, fmt.Sprintf(
					"Tests failed (attempt %d/%d). Fix the issues:\n%s",
					card.RetryCount, card.MaxRetries, truncateStr(testOutput, 4000)))
			}
			a.moveCardToColumn(state, card.ID, ColInProgress)
			_ = saveKanbanState(dir, *state)
			return
		}
		card.ReviewResult = "test_fail_final"
		_ = saveKanbanState(dir, *state)
		return
	}

	// Step 2: Spawn review agent and wait for result
	reviewSessionID := a.spawnReviewAgent(card, dir)
	if reviewSessionID < 0 {
		card.ReviewResult = "review_error"
		_ = saveKanbanState(dir, *state)
		return
	}
	reviewPassed := a.waitForReviewResult(reviewSessionID)

	// Step 3: Handle review failure
	if !reviewPassed {
		if card.RetryCount < card.MaxRetries {
			card.RetryCount++
			card.ReviewResult = "review_fail"
			if card.AgentSessionID > 0 {
				a.AddToQueue(card.AgentSessionID, fmt.Sprintf(
					"Code review failed (attempt %d/%d). Address the review feedback.",
					card.RetryCount, card.MaxRetries))
			}
			a.moveCardToColumn(state, card.ID, ColInProgress)
			_ = saveKanbanState(dir, *state)
			return
		}
		card.ReviewResult = "review_fail_final"
		_ = saveKanbanState(dir, *state)
		return
	}

	// Step 4: Create PR
	card.ReviewResult = "pass"
	prNumber, err := a.createPRForCard(card, dir)
	if err != nil {
		log.Printf("[review] PR creation failed for card %s: %v", card.ID, err)
		card.ReviewResult = "pr_error"
		_ = saveKanbanState(dir, *state)
		return
	}
	card.PRNumber = prNumber
	a.emitOrchestratorEvent("", "pr_created")

	// Step 5: Auto-merge if flagged
	if card.AutoMerge {
		if err := a.autoMergeCard(card, dir); err != nil {
			log.Printf("[review] auto-merge failed for card %s: %v", card.ID, err)
		}
	}

	// Step 6: Move to done and check parent issue
	a.moveCardToColumn(state, card.ID, ColDone)
	_ = saveKanbanState(dir, *state)
	a.emitOrchestratorEvent("", "step_done")
	log.Printf("[review] pipeline complete for card %s, PR #%d", card.ID, prNumber)
	a.checkAndCloseParentIssue(dir, card.ParentIssue, state)
}

// waitForReviewResult polls the review session until it finishes, then checks
// the screen buffer for REVIEW_PASS or REVIEW_FAIL.
func (a *AppService) waitForReviewResult(sessionID int) bool {
	for i := 0; i < 120; i++ { // max 10 minutes (120 * 5s)
		time.Sleep(5 * time.Second)
		a.mu.Lock()
		sess := a.sessions[sessionID]
		a.mu.Unlock()
		if sess == nil {
			return false
		}
		activity := activityString(sess.GetActivity())
		if activity == "done" || activity == "idle" {
			text := sess.Screen.PlainText()
			return strings.Contains(text, "REVIEW_PASS")
		}
	}
	log.Printf("[review] review session %d timed out", sessionID)
	return false
}
