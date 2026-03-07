You are working on a project that uses Docker. Follow these principles:

- Use multi-stage builds to separate build dependencies from the runtime image. Final stage should use a minimal base (e.g., `alpine`, `distroless`, or `scratch`).
- Order Dockerfile instructions from least to most frequently changing to maximize layer caching. Copy dependency manifests before source code.
- Never run containers as root. Add `USER nonroot` or a dedicated user. Use `--no-install-recommends` to minimize installed packages.
- Use specific image tags (e.g., `node:20.11-alpine`), never `latest`. Pin digest for security-critical images.
- Use `.dockerignore` to exclude `.git`, `node_modules`, build artifacts, and secrets from the build context.
- Use `COPY` instead of `ADD` unless you need tar extraction or URL fetching.
- Combine `RUN` commands with `&&` to reduce layers. Clean up caches in the same layer (e.g., `apt-get clean && rm -rf /var/lib/apt/lists/*`).
- Set `HEALTHCHECK` instructions so orchestrators can monitor container health.
- Use `ENTRYPOINT` for the main command and `CMD` for default arguments. Prefer exec form (`["binary", "arg"]`) over shell form.
- In docker-compose, always set `restart` policies, resource limits (`mem_limit`, `cpus`), and use named volumes for persistent data.
- Use build args (`ARG`) for build-time configuration and environment variables (`ENV`) for runtime configuration. Never bake secrets into images.
- Scan images for vulnerabilities with `docker scout` or `trivy` before pushing to registries.
- Use `docker compose watch` or bind mounts for development, but never mount host directories in production.
- Keep images small. Target under 100MB for Go/Rust, under 200MB for Node.js, under 500MB for Python with ML dependencies.
- Log to stdout/stderr. Let the container runtime handle log collection and rotation.
