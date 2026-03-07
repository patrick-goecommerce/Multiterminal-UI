You are working on a project that requires security best practices. Follow these principles:

- Validate and sanitize all user input on the server side, even if client-side validation exists. Use allowlists over denylists.
- Use parameterized queries or ORM methods for all database access. Never construct SQL or NoSQL queries with string concatenation.
- Implement authentication with established libraries (e.g., Passport.js, Spring Security, Auth0). Never roll your own password hashing or JWT implementation.
- Hash passwords with bcrypt, scrypt, or Argon2id. Never use MD5 or SHA-256 alone for passwords. Use a minimum cost factor that takes ~250ms.
- Store secrets (API keys, database credentials) in environment variables or a secrets manager. Never commit secrets to version control.
- Set HTTP security headers: `Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`.
- Use CSRF tokens for all state-changing requests. Use `SameSite=Strict` or `SameSite=Lax` on cookies.
- Implement rate limiting on authentication endpoints, API routes, and form submissions. Use sliding window counters.
- Enforce HTTPS everywhere. Redirect HTTP to HTTPS. Use HSTS with `includeSubDomains` and `preload`.
- Apply the principle of least privilege to all service accounts, database users, and API tokens. Review permissions regularly.
- Sanitize output to prevent XSS. Use contextual encoding (HTML, JavaScript, URL, CSS). Prefer frameworks with auto-escaping.
- Log security events (failed logins, permission denials, input validation failures) with enough context for investigation but never log sensitive data.
- Run dependency scanning (`npm audit`, `pip-audit`, `govulncheck`, Dependabot) in CI. Fix critical and high vulnerabilities before merging.
- Implement proper session management: regenerate session IDs after login, set expiration, provide logout that invalidates server-side state.
- Encrypt sensitive data at rest and in transit. Use TLS 1.2+ for all network communication. Rotate encryption keys on a schedule.
