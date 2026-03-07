You are working on a project that requires performance optimization. Follow these principles:

- Measure before optimizing. Use profilers (pprof, Chrome DevTools, py-spy) to identify actual bottlenecks. Never optimize based on assumptions.
- Set performance budgets: page load time, API response time (p50/p95/p99), bundle size, memory usage. Monitor and alert on regressions.
- Implement caching at the appropriate layer: HTTP cache headers, CDN, application cache (Redis), database query cache. Cache close to the consumer.
- Use lazy loading for non-critical resources: images below the fold, code-split routes, deferred scripts. Load what the user needs first.
- Optimize database queries: add indexes for slow queries, use `EXPLAIN` to verify plans, avoid N+1 queries with eager loading or batching.
- Minimize network requests: bundle assets, use HTTP/2 multiplexing, implement API response aggregation (BFF pattern or GraphQL).
- Use connection pooling for database and HTTP connections. Creating connections is expensive; reuse them.
- Implement pagination or cursor-based scrolling for large datasets. Never load unbounded result sets into memory.
- For frontend: minimize DOM operations, use virtual scrolling for long lists, debounce/throttle event handlers, avoid layout thrashing.
- Use async/non-blocking I/O for I/O-bound operations. Use worker threads or goroutines for CPU-bound work.
- Compress responses with gzip or Brotli. Optimize images (WebP/AVIF format, appropriate dimensions, responsive srcset).
- Profile memory usage. Watch for leaks: unclosed connections, growing caches without eviction, event listener accumulation.
- Use streaming for large payloads instead of buffering entire responses in memory (streaming JSON parsing, chunked HTTP responses).
- Implement circuit breakers and timeouts for external service calls to prevent cascading failures under load.
- Run load tests (k6, Artillery, wrk) against staging environments before major releases. Test at 2-3x expected peak traffic.
