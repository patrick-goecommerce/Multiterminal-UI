You are working on a project that requires accessibility (a11y) compliance. Follow these principles:

- Target WCAG 2.1 Level AA compliance at minimum. Test with automated tools (axe, Lighthouse) and manual screen reader testing.
- Use semantic HTML elements (`nav`, `main`, `article`, `button`, `header`, `footer`) instead of generic `div` and `span` with ARIA roles.
- Every interactive element must be keyboard accessible. Ensure logical tab order, visible focus indicators, and no keyboard traps.
- All images must have `alt` text. Decorative images use `alt=""`. Complex images (charts, diagrams) need detailed descriptions.
- Maintain color contrast ratios: 4.5:1 for normal text, 3:1 for large text (18pt+ or 14pt+ bold). Use tools like Colour Contrast Analyser.
- Never convey information through color alone. Use icons, patterns, text labels, or other visual indicators alongside color.
- Add ARIA attributes only when semantic HTML is insufficient. Prefer native elements (`button` over `div role="button"`). Incorrect ARIA is worse than no ARIA.
- Implement skip navigation links ("Skip to main content") as the first focusable element on each page.
- Form inputs must have associated `label` elements (using `for`/`id` or wrapping). Use `aria-describedby` for help text and error messages.
- Provide clear, specific error messages positioned near the relevant form field. Use `aria-live="polite"` for dynamic error announcements.
- Support text resizing up to 200% without loss of content or functionality. Use relative units (`rem`, `em`) not fixed pixels for text.
- Ensure all interactive components have accessible names. Use `aria-label` or `aria-labelledby` when visible text is insufficient.
- Test with actual screen readers: NVDA or JAWS on Windows, VoiceOver on macOS/iOS, TalkBack on Android. Automated tools catch only ~30% of issues.
- Handle focus management for modals, dialogs, and dynamic content: trap focus in modals, return focus on close, announce new content.
- Provide captions for video and transcripts for audio content. Ensure media players have accessible controls.
