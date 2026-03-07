You are working on a project that uses PostgreSQL. Follow these principles:

- Always write migrations as idempotent, versioned SQL files. Use `IF NOT EXISTS` / `IF EXISTS` guards.
- Never modify or delete a migration that has been applied. Create a new migration to alter schema.
- Add indexes for columns used in `WHERE`, `JOIN`, and `ORDER BY` clauses. Prefer partial indexes when filtering on a subset of rows.
- Use `EXPLAIN ANALYZE` to verify query plans before and after optimization. Watch for sequential scans on large tables.
- Prefer `EXISTS` over `IN` for correlated subqueries. Use CTEs for readability but be aware they are optimization fences in older PostgreSQL versions (< 12).
- Use connection pooling (e.g., PgBouncer) in production. Never hold transactions open longer than necessary.
- Use parameterized queries exclusively. Never interpolate user input into SQL strings.
- Normalize to 3NF by default. Denormalize deliberately with documented justification when read performance requires it.
- Use `BIGINT` or `UUID` for primary keys. Avoid `SERIAL` in new projects; prefer `GENERATED ALWAYS AS IDENTITY`.
- Add `NOT NULL` constraints by default. Allow `NULL` only with explicit justification.
- Use `TIMESTAMP WITH TIME ZONE` for all temporal columns. Store times in UTC.
- Add `CHECK` constraints for domain rules (e.g., `price > 0`). Use foreign keys to enforce referential integrity.
- Use `pg_stat_statements` to identify slow queries. Target p95 latency, not just averages.
- For bulk inserts, use `COPY` or multi-row `INSERT` with `RETURNING`. Batch sizes of 1000-5000 rows are typical.
- Use advisory locks or `SELECT FOR UPDATE SKIP LOCKED` for queue-like workloads instead of polling with row locks.
- Test migrations against a copy of production data volume. A migration fast on 100 rows may lock a 10M-row table for minutes.
