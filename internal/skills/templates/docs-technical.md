You are working on a project that requires technical documentation. Follow these principles:

- Write documentation for your audience: API docs for integrators, guides for users, ADRs for future maintainers. Adjust depth and tone accordingly.
- Use Architecture Decision Records (ADRs) for significant technical decisions. Include context, decision, consequences, and status (proposed/accepted/deprecated).
- Structure documentation with a clear hierarchy: overview -> getting started -> guides -> API reference -> troubleshooting. Users should find answers within 3 clicks.
- Every public API must have documentation: endpoint URL, method, request/response schemas, authentication, error codes, and at least one working example.
- Include runnable code examples. Examples that do not compile or run erode trust. Test code examples in CI when possible.
- Keep a CHANGELOG following the Keep a Changelog format: Added, Changed, Deprecated, Removed, Fixed, Security. Update it with every user-facing change.
- Use consistent terminology throughout. Define domain terms in a glossary. Never use two different words for the same concept.
- Document environment setup and prerequisites explicitly. Include exact versions, OS-specific instructions, and common setup errors.
- Write commit messages and PR descriptions as documentation. Future developers will read `git log` to understand why changes were made.
- Use diagrams (Mermaid, PlantUML, or simple ASCII) for architecture, data flow, and sequence diagrams. A diagram replaces paragraphs of text.
- Document error messages and their solutions. When users encounter an error, they search for the exact error text.
- Keep docs close to the code they describe. Inline JSDoc/GoDoc/docstrings for functions, README in each package/module directory.
- Review documentation during code review. If the code changes behavior, the docs must change too. Treat stale docs as bugs.
- Write troubleshooting guides based on actual support requests. FAQ sections should answer real questions, not imagined ones.
- Date and version your documentation. Readers need to know if the docs match their version of the software.
