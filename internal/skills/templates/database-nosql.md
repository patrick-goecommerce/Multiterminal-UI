You are working on a project that uses NoSQL databases (MongoDB, Redis). Follow these principles:

- Model data by access patterns, not by entities. Embed documents when data is read together; reference when data is shared or updated independently.
- Avoid unbounded arrays in MongoDB documents. If an array can grow without limit, use a separate collection with a foreign key reference.
- Always create indexes for fields used in queries. Use `explain()` to verify index usage. Compound indexes must match query field order (ESR rule: Equality, Sort, Range).
- Use MongoDB aggregation pipelines for complex queries. Place `$match` and `$project` stages early to reduce documents flowing through the pipeline.
- Set appropriate write concern and read preference for your consistency requirements. Default to `w: majority` for critical writes.
- Use MongoDB transactions only when truly needed (multi-document atomicity). Single-document operations are already atomic.
- For Redis, choose the right data structure: Strings for simple K/V, Hashes for objects, Sorted Sets for leaderboards/rankings, Streams for event logs.
- Set TTLs on Redis keys by default. Every key without a TTL is a potential memory leak.
- Use Redis pipelines to batch multiple commands and reduce round-trip latency.
- Never use `KEYS *` in production Redis. Use `SCAN` for iteration.
- Implement cache-aside (lazy loading) pattern: read from cache first, fetch from DB on miss, populate cache. Set reasonable TTLs.
- Use Redis Lua scripts for atomic multi-step operations instead of client-side transactions.
- Design MongoDB schemas with a maximum document size of 16MB in mind. Store large blobs in GridFS or object storage.
- Use change streams (MongoDB) or keyspace notifications (Redis) for reactive patterns instead of polling.
- Always handle connection failures gracefully with retry logic and circuit breakers.
