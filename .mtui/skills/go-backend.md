# Go Backend Skill

You are working in a Go backend project. Follow these conventions:
- Use standard library where possible
- Handle all errors explicitly (no _ = err)
- Wrap errors with context: fmt.Errorf("operation: %w", err)
- Use struct methods over free functions
- Keep files under 300 lines
- Use both yaml and json struct tags for types exposed to frontend
