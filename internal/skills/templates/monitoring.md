You are setting up or improving observability and monitoring. Follow these principles:

- **Three pillars:** implement metrics, logs, and traces. Metrics for alerting and dashboards, logs for debugging, traces for request flow across services. All three are needed for full observability.
- **Structured logging:** log in JSON format with consistent fields: timestamp, level, service, request_id, user_id. Never log passwords, tokens, PII, or credit card numbers.
- **Log levels:** ERROR for actionable failures, WARN for degraded but functional state, INFO for significant business events, DEBUG for development. Production should run at INFO level.
- **Metrics:** use RED method for services (Rate, Errors, Duration) and USE method for resources (Utilization, Saturation, Errors). Instrument at application boundaries: HTTP handlers, DB queries, external calls.
- **Prometheus/OpenTelemetry:** use counters for totals (requests_total), histograms for latencies (request_duration_seconds), gauges for current state (active_connections). Follow naming conventions: `<namespace>_<name>_<unit>`.
- **Distributed tracing:** propagate trace context (W3C TraceContext or B3) across all service calls. Add span attributes for business context (user_id, order_id). Sample at 1-10% in production for cost control.
- **Alerting:** alert on symptoms (error rate > 1%, p99 latency > 500ms), not causes. Use multi-window burn rate for SLO-based alerting. Every alert must have a runbook link.
- **Dashboards:** create a service overview dashboard (golden signals), a detailed per-service dashboard, and business KPI dashboards. Use consistent layouts across services.
- **SLOs/SLIs:** define Service Level Objectives for critical user journeys (e.g., 99.9% availability, p95 < 200ms). Measure with Service Level Indicators. Track error budgets.
- **Health checks:** implement `/health` (liveness) and `/ready` (readiness) endpoints. Liveness: process is running. Readiness: can serve traffic (DB connected, caches warm).
- **Incident response:** centralize logs and metrics (Grafana, Datadog, ELK). Set up PagerDuty/Opsgenie for on-call rotation. Run blameless postmortems for every P1/P2 incident.
- **Cost management:** set retention policies (7d hot, 30d warm, 90d cold). Downsample old metrics. Use log levels and sampling to control volume. Monitor observability spend.
- **Synthetic monitoring:** implement canary checks for critical endpoints. Run synthetic transactions from multiple regions. Alert on availability and latency degradation before users notice.
