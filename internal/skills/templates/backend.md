You are working on a backend project. Follow these principles, adapting to the specific language (Go, Node, Python, Rust, Java, C#, Ruby, PHP):

- **Error handling:** never swallow errors. Add context when propagating (Go: `fmt.Errorf("op: %w", err)`, Python: `raise XError(...) from e`). Distinguish between client errors (4xx) and server errors (5xx).
- **API design:** use resource-oriented URLs. Apply correct HTTP methods and status codes. Return structured error responses with error codes. Use consistent pagination and filtering patterns.
- **Input validation:** validate all external input at system boundaries. Use allowlists over denylists. Apply strong typing (Go structs, Pydantic, Zod) for request/response shapes.
- **Dependency injection:** design for testability. Accept interfaces/abstractions, return concrete types. Avoid global state and singletons.
- **Concurrency:** understand your runtime's concurrency model. Go: goroutines + channels/mutexes. Node: async/await + event loop. Python: asyncio or threading. Never hold locks during I/O.
- **Database access:** use parameterized queries exclusively. Use connection pooling. Keep transactions short. Handle migrations with versioned, idempotent scripts.
- **Logging:** use structured logging (slog, winston, structlog). Log at appropriate levels: Error for failures, Info for operations, Debug for troubleshooting. Never log secrets or PII.
- **Configuration:** use environment variables or config files. Validate config at startup, fail fast on missing values. Never commit secrets to version control.
- **Testing:** write table-driven/parameterized tests. Mock external dependencies in unit tests, use real instances in integration tests. Test error paths explicitly.
- **Resource cleanup:** close files, connections, and response bodies immediately after use. Use defer (Go), context managers (Python), try-with-resources (Java), using (C#).
- **Performance:** profile before optimizing. Use caching strategically (Redis, in-memory). Implement pagination for list endpoints. Use bulk operations for batch processing.
- **File organization:** keep files under 300 lines. Group by responsibility. One package/module per concern. Avoid circular dependencies.
