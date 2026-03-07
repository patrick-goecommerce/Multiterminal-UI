You are working on a frontend project. Follow these principles, adapting to the specific framework in use (React, Vue, Svelte, Angular, etc.):

- **Component architecture:** keep components small and focused (under 150 lines). Extract reusable logic into composables/hooks/stores. Props down, events up.
- **State management:** prefer local component state. Lift state up before reaching for global stores. Use framework-native solutions (React Context, Pinia, Svelte stores) before external libraries.
- **Reactivity discipline:** understand your framework's reactivity model. Avoid unnecessary re-renders (React: `useMemo`/`useCallback`, Vue: `computed`, Svelte: reactive declarations). Profile before optimizing.
- **TypeScript:** enable `strict` mode. Define prop types explicitly. Avoid `any`; use `unknown` and narrow with type guards.
- **CSS strategy:** use scoped styles (CSS Modules, `<style scoped>`, Svelte scoped) or utility-first (Tailwind). Prefer CSS custom properties for theming. Use `rem` for sizing, `px` only for borders.
- **Responsive design:** mobile-first approach. Use CSS Grid for page layouts, Flexbox for component layouts. Test at standard breakpoints (320px, 768px, 1024px, 1440px).
- **Accessibility:** all interactive elements must be keyboard-navigable. Use semantic HTML (`button`, `nav`, `main`, `article`). Add `aria-label` only when visible text is insufficient. Test with screen readers.
- **Forms:** use controlled/bound inputs. Validate both client-side (instant feedback) and server-side (security). Use established form libraries (react-hook-form, VeeValidate) for complex forms.
- **Data fetching:** fetch server-side when possible (SSR/SSG). Client-side: use SWR/TanStack Query patterns with caching, deduplication, and error/loading states. Always handle loading and error UI.
- **Performance:** lazy-load routes and heavy components. Optimize images (WebP, `srcset`, lazy loading). Minimize bundle size — analyze with bundlephobia or webpack-bundle-analyzer.
- **Testing:** test behavior, not implementation. Use Testing Library conventions (query by role, label, text). Mock API calls at the network layer (msw), not module level.
- **Error handling:** use error boundaries (React) or error pages (Next.js, Nuxt, SvelteKit). Show user-friendly messages. Log to monitoring service.
- **Mobile (React Native / Flutter):** use platform-specific patterns. Prefer FlatList/ListView for long lists. Handle offline state. Test on real devices, not just simulators.
