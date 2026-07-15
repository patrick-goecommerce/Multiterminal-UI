You are working on a project that requires comprehensive testing. Follow these principles:

- Follow the testing pyramid: many unit tests, fewer integration tests, minimal e2e tests. Each level tests different concerns.
- Write tests that are independent and isolated. No test should depend on the execution order or state from another test. Use setup/teardown properly.
- Name tests descriptively: `test_returns_404_when_user_not_found` not `test_get_user_3`. The name should describe the scenario and expected behavior.
- Use the Arrange-Act-Assert (AAA) pattern. Separate test setup, action, and verification clearly.
- Mock external dependencies (APIs, databases, filesystems) in unit tests. Use real instances in integration tests.
- Prefer dependency injection over monkey-patching for testability. Design code with testing in mind from the start.
- Use factory functions or builders to create test data. Avoid fixtures that are shared and modified across tests.
- Test edge cases: empty inputs, null values, boundary conditions, error paths, concurrent access, and large inputs.
- Write integration tests for critical paths: authentication flow, payment processing, data pipeline end-to-end.
- Use test coverage as a guide, not a target. 80% coverage with meaningful assertions beats 100% coverage with trivial checks.
- For e2e tests, use stable selectors (data-testid attributes) not CSS classes or text content that may change.
- Run fast unit tests on every commit. Run slower integration and e2e tests in CI or on pre-merge.
- Use table-driven tests (Go), parameterized tests (pytest, JUnit), or `test.each` (Jest) for testing multiple inputs against the same logic.
- Test error handling explicitly. Verify that errors are thrown, logged, and propagated correctly.
- Implement contract tests for service boundaries (API contracts between frontend and backend, microservice interfaces).
- When fixing a bug, write a failing test first that reproduces the bug, then fix the code. This prevents regressions.
