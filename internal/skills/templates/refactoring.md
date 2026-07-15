You are working on code that needs restructuring. Follow these refactoring principles:

- **Red-Green-Refactor:** ensure tests pass before refactoring. Make structural changes without altering behavior. Run tests after every change. Never refactor and add features simultaneously.
- **Extract Method:** if a block of code needs a comment to explain it, extract it into a well-named function. The function name replaces the comment.
- **Single Responsibility:** each function/class/module should have one reason to change. Split files over 300 lines. Separate data access, business logic, and presentation.
- **DRY with judgment:** extract duplication only when the duplicated code changes for the same reason. Three similar lines are better than a premature abstraction. Apply the Rule of Three.
- **SOLID principles:** Open/Closed (extend via composition, not modification), Liskov (subtypes must be substitutable), Interface Segregation (small focused interfaces), Dependency Inversion (depend on abstractions).
- **Reduce complexity:** flatten nested conditions with early returns/guard clauses. Replace complex conditionals with polymorphism or strategy pattern. Limit function parameters to 3-4.
- **Naming:** rename unclear variables, functions, and types to express intent. Code should read like prose. Avoid abbreviations that aren't universally understood.
- **Remove dead code:** delete unused functions, variables, imports, and commented-out code. Version control preserves history — you can always recover it.
- **Simplify dependencies:** break circular dependencies. Move shared code to a common package. Prefer composition over inheritance.
- **Data structures:** choose the right data structure for the access pattern. Maps for lookup, arrays for iteration, sets for membership. Profile before optimizing.
- **Incremental refactoring:** make small, safe changes. Commit after each working step. Use feature flags to decouple deploy from release when refactoring live code.
- **Code smells to watch for:** long methods, large classes, feature envy, data clumps, primitive obsession, shotgun surgery. Address root causes, not symptoms.
