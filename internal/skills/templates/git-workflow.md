You are working in a project that follows structured Git practices. Follow these principles:

- **Branch strategy:** use feature branches off the main development branch. Name branches descriptively: `feat/user-auth`, `fix/login-redirect`, `chore/update-deps`.
- **Conventional commits:** format: `type(scope): description`. Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`. Keep subject under 72 chars.
- **Commit discipline:** each commit should be atomic — one logical change per commit. Commits should compile and pass tests individually. Never commit generated files or secrets.
- **Pull requests:** keep PRs focused and reviewable (< 400 lines changed). Write a clear description with context, approach, and test plan. Link related issues.
- **Merge strategy:** prefer squash-merge for feature branches (clean history). Use merge commits for long-lived branches. Rebase only local/unpushed commits.
- **Code review:** review for correctness, security, performance, and maintainability. Be specific in feedback. Approve only when all concerns are addressed.
- **Conflict resolution:** rebase onto target branch before merging. Resolve conflicts carefully — never blindly accept "ours" or "theirs". Test after resolving.
- **Worktrees:** use `git worktree` for parallel work on multiple branches. Keeps working directory clean. Ideal for hotfixes while mid-feature.
- **Tags:** use semantic versioning (`v1.2.3`). Tag releases from the main branch. Use annotated tags (`git tag -a`) with release notes.
- **Stashing:** use `git stash` with descriptive messages (`git stash push -m "WIP: login form"`). Apply and drop stashes promptly; don't let them accumulate.
- **History hygiene:** use `git rebase -i` to clean up local commits before pushing. Squash fixup commits. Never rewrite published history.
- **Hooks:** use pre-commit hooks for linting and formatting. Use commit-msg hooks for conventional commit validation. Don't skip hooks without good reason.
