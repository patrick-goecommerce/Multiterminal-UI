You are working on CSS/styling for a web project. Follow these principles:

- **Utility-first with Tailwind CSS:** compose styles from utility classes in markup. Extract repeated patterns into components, not CSS classes. Use `@apply` only in component-level stylesheets for truly reusable atomic patterns, never in global CSS.
- **Tailwind configuration:** extend the default theme in `tailwind.config.js` rather than overriding. Define design tokens (colors, spacing, fonts) in `theme.extend` so defaults remain available.
- **Responsive design:** design mobile-first. Use Tailwind's responsive prefixes (`sm:`, `md:`, `lg:`) or CSS media queries with `min-width` breakpoints. Never use `max-width` queries as the primary approach.
- **CSS custom properties:** use `--var-name` for theming, dynamic values, and component-level customization. Define global tokens on `:root`. Override in scoped selectors for theme variants (e.g., `.dark { --bg: #1a1a1a; }`).
- **Layout:** use CSS Grid for two-dimensional layouts (page structure, card grids). Use Flexbox for one-dimensional alignment (navbars, inline elements). Avoid floats and absolute positioning for layout.
- **Spacing and sizing:** use a consistent scale (4px/0.25rem increments). Never use arbitrary pixel values — map to your design system's spacing tokens. Use `rem` for typography, `em` for component-relative spacing.
- **Typography:** define a type scale with limited sizes (e.g., 6-8 levels). Set `line-height` relative to font size. Use `font-display: swap` for web fonts to prevent layout shift.
- **Colors:** define a semantic color palette (primary, secondary, surface, error) rather than using raw hex values. Support dark mode via CSS custom properties or Tailwind's `dark:` variant.
- **Animations:** prefer CSS transitions for state changes (hover, active, disabled). Use `@keyframes` for complex or looping animations. Respect `prefers-reduced-motion` — disable or simplify non-essential animations.
- **Specificity:** keep selectors flat — max 2 levels of nesting. Never use `!important` except to override third-party library styles. Use BEM naming (`.block__element--modifier`) if writing custom CSS without a utility framework.
- **Performance:** avoid layout-triggering properties in animations (use `transform` and `opacity`). Use `will-change` sparingly and only on elements that actually animate. Minimize CSS bundle size by purging unused styles.
- **Accessibility:** ensure color contrast meets WCAG AA (4.5:1 for text, 3:1 for large text). Never rely on color alone to convey information — add icons or text. Focus styles must be visible — never set `outline: none` without a replacement.
- **Container queries:** use `@container` for component-level responsive design when the component's width matters more than the viewport width. Define containment with `container-type: inline-size`.
- **Organization:** group styles by component, not by property type. Co-locate component styles with component files. Use CSS layers (`@layer`) to control cascade order across base, components, and utilities.
