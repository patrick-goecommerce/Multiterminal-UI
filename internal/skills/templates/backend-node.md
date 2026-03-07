You are working on a Node.js backend project. Follow these principles:

- **Async/await everywhere.** Never use raw callbacks. Always `await` promises and wrap async route handlers to catch rejected promises. Use `Promise.all` for independent concurrent operations, `Promise.allSettled` when partial failure is acceptable.
- **Error handling:** create custom error classes extending `Error` with `statusCode` and `code` properties. Use a centralized error-handling middleware in Express (`(err, req, res, next)`). Never swallow errors silently — log and re-throw or respond.
- **Express middleware:** keep middleware functions focused on one concern (auth, validation, logging). Apply middleware in order: logging, security (helmet, cors), parsing, auth, routes, error handler. Use `express.Router()` to organize routes by resource.
- **Input validation:** validate all request input (body, params, query) at the controller layer using Zod, Joi, or similar. Return 400 with specific error messages. Never trust client input — sanitize before database queries.
- **Project structure:** organize by feature/domain, not by technical layer. Example: `users/users.controller.ts`, `users/users.service.ts`, `users/users.repository.ts`. Keep controllers thin — delegate logic to services.
- **Environment configuration:** use `dotenv` for local development. Access config through a validated config module, never raw `process.env` scattered through code. Fail fast on missing required variables at startup.
- **Database access:** use an ORM (Prisma, Drizzle) or query builder (Knex) with migrations. Never concatenate SQL strings — use parameterized queries. Wrap multi-step operations in transactions.
- **Streaming:** use Node.js streams for large file processing, CSV exports, and real-time data. Pipe streams with proper error handling (`pipeline` from `stream/promises`). Use `Transform` streams for data processing.
- **Security:** use `helmet` for HTTP headers, `cors` with explicit origin allowlists, rate limiting on auth endpoints. Hash passwords with bcrypt (cost factor 12+). Use JWT with short expiration and refresh tokens.
- **TypeScript:** enable `strict` mode. Define request/response types. Use `unknown` over `any`. Type Express request with custom interfaces for typed `req.user`, `req.params`, etc.
- **Logging:** use a structured logger (pino, winston) with request IDs for tracing. Log at appropriate levels: `error` for failures, `info` for request/response lifecycle, `debug` for development. Never log sensitive data (passwords, tokens).
- **Testing:** use Jest or Vitest. Unit-test services with mocked dependencies. Integration-test API endpoints with `supertest`. Use factory functions for test data, not fixtures. Test error paths, not just happy paths.
- **Graceful shutdown:** handle `SIGTERM` and `SIGINT`. Stop accepting new connections, finish in-flight requests, close database connections, then exit. Set a timeout to force exit if cleanup stalls.
- **Performance:** avoid synchronous operations on the event loop. Use worker threads for CPU-intensive tasks. Cache expensive computations with Redis or in-memory LRU. Set appropriate timeouts on outgoing HTTP requests.
