You are working on a React/Next.js project. Follow these principles:

- **Functional components only.** Use hooks (`useState`, `useEffect`, `useMemo`, `useCallback`) instead of class components.
- **Custom hooks** for reusable logic. Extract shared stateful behavior into `useXxx` hooks in a `hooks/` directory.
- **Component structure:** props interface first, then hook calls, then derived values, then handlers, then JSX return. Keep components under 150 lines.
- **State management:** prefer local state and lifting state up. Reach for Context only for truly global state (auth, theme). Use Zustand or Jotai for complex cross-cutting state; avoid Redux unless already established.
- **Avoid unnecessary re-renders.** Memoize expensive computations with `useMemo`, stabilize callbacks with `useCallback`, and wrap child components in `React.memo` only when profiling shows a bottleneck.
- **Effects discipline:** every `useEffect` must have a correct dependency array. Never suppress the exhaustive-deps lint. Use cleanup functions for subscriptions, timers, and abort controllers.
- **Next.js SSR/SSG:** prefer `getStaticProps` (Pages Router) or React Server Components (App Router) for data that doesn't change per-request. Use `getServerSideProps` / server actions only when freshness is critical. Never fetch data in `useEffect` if it can be fetched server-side.
- **File conventions (App Router):** `page.tsx` for routes, `layout.tsx` for shared layouts, `loading.tsx` for Suspense fallbacks, `error.tsx` for error boundaries. Co-locate components next to their route.
- **Forms:** use controlled components. For complex forms, use `react-hook-form` with Zod validation schemas.
- **Testing with React Testing Library:** test behavior, not implementation. Query by role, label, or text — never by test ID unless no semantic alternative exists. Use `userEvent` over `fireEvent`. Mock API calls at the network layer (`msw`), not at the module level.
- **Error boundaries:** wrap route-level and feature-level subtrees. Log errors to your monitoring service in `componentDidCatch` or the `onError` callback.
- **Accessibility:** all interactive elements must be keyboard-navigable. Use semantic HTML (`button`, `nav`, `main`). Add `aria-label` only when visible text is insufficient.
- **Imports:** use path aliases (`@/components/...`) over deep relative paths. Barrel files (`index.ts`) are fine for public API surfaces but avoid re-exporting everything.
- **TypeScript:** enable `strict` mode. Define prop types as interfaces (not `type` aliases) when they may be extended. Avoid `any`; use `unknown` and narrow with type guards.
