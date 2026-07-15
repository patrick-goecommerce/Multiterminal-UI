You are working on a project with DevOps/infrastructure concerns. Follow these principles:

- **Docker:** use multi-stage builds. Final stage on minimal base (alpine, distroless). Never run as root. Pin image tags (never `latest`). Keep images small.
- **Docker — Best practices:** order Dockerfile layers from least to most changing. Use `.dockerignore`. Combine `RUN` commands. Set `HEALTHCHECK`. Log to stdout/stderr.
- **Docker Compose:** set restart policies, resource limits, named volumes. Use `docker compose watch` for dev. Never mount host dirs in production.
- **Kubernetes:** use declarative manifests or Helm charts. Set resource requests AND limits. Use liveness, readiness, and startup probes. Never use `latest` tag.
- **K8s — Security:** use NetworkPolicies to restrict traffic. Run pods as non-root. Use RBAC with least privilege. Scan images in CI. Store secrets in Sealed Secrets or external vault.
- **CI/CD — Pipelines:** keep pipelines fast (< 10min). Cache dependencies. Run linting and tests in parallel. Use matrix builds for multi-platform/version testing.
- **CI/CD — GitHub Actions:** pin action versions by SHA. Use `concurrency` to cancel outdated runs. Store secrets in GitHub Secrets, never in code. Use OIDC for cloud auth.
- **Terraform/IaC:** use modules for reusable infrastructure. Lock state files. Use `plan` before `apply`. Pin provider versions. Use workspaces or separate state files per environment.
- **Terraform — State:** use remote state backends (S3, GCS) with locking. Never edit state files manually. Use `import` for existing resources. Review plans carefully.
- **AWS/Cloud:** use least-privilege IAM policies. Enable CloudTrail logging. Use managed services over self-hosted. Tag all resources consistently.
- **Monitoring:** implement health endpoints. Set up alerts for error rates, latency, and resource usage. Use structured logging with correlation IDs.
- **Security:** scan dependencies and images in CI. Rotate credentials on schedule. Use HTTPS everywhere. Follow the principle of least privilege for all service accounts.
