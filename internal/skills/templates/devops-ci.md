You are working on a project with CI/CD pipelines. Follow these principles:

- Keep pipelines fast. Target under 10 minutes for PR checks. Use parallelism, caching, and selective test execution.
- Cache dependency directories (e.g., `node_modules`, `.gradle`, `~/.cache/pip`) between runs. Use hash-based cache keys tied to lockfiles.
- Use matrix builds to test across multiple OS/language/dependency versions. Fail fast with `fail-fast: true` on matrix strategies.
- Run linting and formatting checks as the first job. They are cheap and catch issues early.
- Split pipelines into stages: lint -> test -> build -> deploy. Use job dependencies to skip later stages on failure.
- Use GitHub Actions reusable workflows or GitLab CI includes to share pipeline logic across repositories.
- Pin action versions to full SHAs, not tags (e.g., `actions/checkout@a1b2c3d` not `actions/checkout@v4`). Tags can be moved.
- Store secrets in the CI platform's secret manager, never in pipeline files. Use environment-scoped secrets for deploy credentials.
- Run security scanning (SAST, dependency audit) in CI. Use `npm audit`, `pip-audit`, `govulncheck`, or Snyk.
- Use concurrency groups to cancel redundant runs when new commits are pushed to the same branch.
- Implement branch protection rules: require passing CI, code review, and up-to-date branches before merge.
- For deployment pipelines, use environment approvals for production. Deploy to staging automatically, production manually.
- Upload test results and coverage reports as artifacts. Use status checks to enforce minimum coverage thresholds.
- Use `GITHUB_TOKEN` permissions with least privilege. Set `permissions:` block explicitly in workflow files.
- Test pipeline changes in feature branches before merging. Use `workflow_dispatch` for manual testing of deploy workflows.
