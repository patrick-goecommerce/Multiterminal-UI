You are working on a project with database components (SQL and/or NoSQL). Follow these principles:

- **SQL — Migrations:** write idempotent, versioned migration files. Use `IF NOT EXISTS` / `IF EXISTS` guards. Never modify applied migrations; create new ones.
- **SQL — Indexing:** add indexes for `WHERE`, `JOIN`, and `ORDER BY` columns. Use partial indexes for filtered subsets. Use `EXPLAIN ANALYZE` to verify query plans.
- **SQL — Schema design:** normalize to 3NF by default. Denormalize deliberately with documented justification. Use `NOT NULL` by default. Add `CHECK` and foreign key constraints.
- **SQL — Types:** use `BIGINT` or `UUID` for primary keys. Use `TIMESTAMP WITH TIME ZONE` for temporal columns (store in UTC). Avoid `SERIAL`; prefer identity columns.
- **SQL — Performance:** use connection pooling (PgBouncer, HikariCP). Keep transactions short. Use `COPY` or multi-row `INSERT` for bulk operations. Batch 1000-5000 rows.
- **SQL — Queries:** prefer `EXISTS` over `IN` for subqueries. Use CTEs for readability. Use parameterized queries exclusively — never interpolate user input.
- **NoSQL — Data modeling:** design documents around query patterns, not entity relationships. Embed related data that is read together. Reference data that changes independently.
- **NoSQL — MongoDB:** use schema validation for critical collections. Create compound indexes matching query patterns. Use `$lookup` sparingly; denormalize instead.
- **NoSQL — Redis:** use appropriate data structures (strings for cache, sorted sets for leaderboards, streams for event logs). Set TTL on cache entries. Use pipelining for batch operations.
- **NoSQL — Consistency:** understand your database's consistency model (eventual vs. strong). Use transactions when atomicity is required. Design for idempotent writes.
- **General — Testing:** test migrations against production-volume data. A migration fast on 100 rows may lock a 10M-row table. Use factory functions for test data; avoid shared fixtures.
- **General — Monitoring:** track query latency (p95, not just average). Use `pg_stat_statements` or equivalent. Set alerts for slow queries and connection pool exhaustion.
