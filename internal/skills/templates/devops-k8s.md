You are working on a project that uses Kubernetes. Follow these principles:

- Always set resource `requests` and `limits` for CPU and memory. Requests affect scheduling; limits prevent noisy-neighbor issues. Start with requests = limits and tune from there.
- Define `readinessProbe` and `livenessProbe` for every container. Use `startupProbe` for slow-starting apps to avoid premature restarts.
- Use `Deployments` for stateless workloads, `StatefulSets` for stateful ones. Never use bare Pods in production.
- Set `PodDisruptionBudgets` to ensure availability during node drains and cluster upgrades.
- Use `Namespaces` to isolate environments and teams. Apply `ResourceQuotas` and `LimitRanges` per namespace.
- Store configuration in `ConfigMaps` and secrets in `Secrets` (or external secret managers like Vault). Never hardcode configuration in manifests.
- Use Helm charts for reusable, parameterized deployments. Keep `values.yaml` well-documented with sensible defaults.
- Apply `NetworkPolicies` to restrict pod-to-pod traffic. Default-deny ingress and egress, then allow explicitly.
- Use RBAC with least-privilege principles. Create service accounts per workload, never use the default service account.
- Set `securityContext` on pods and containers: `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, drop all capabilities.
- Use `topologySpreadConstraints` or `podAntiAffinity` to spread replicas across nodes and zones.
- Implement `HorizontalPodAutoscaler` based on CPU, memory, or custom metrics. Set `minReplicas >= 2` for production workloads.
- Use labels consistently for selection and organization: `app.kubernetes.io/name`, `app.kubernetes.io/version`, `app.kubernetes.io/component`.
- Use `kustomize` overlays or Helm value files to manage environment differences (dev/staging/prod) without duplicating manifests.
- Run `kubectl diff` before applying changes. Use GitOps (ArgoCD, Flux) for production deployments instead of manual `kubectl apply`.
