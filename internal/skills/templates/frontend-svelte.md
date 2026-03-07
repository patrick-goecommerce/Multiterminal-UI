You are working on a Svelte project. Follow these principles:

- **Svelte 4:** use `let` declarations for reactive state. Reassignment triggers reactivity — mutating objects/arrays does not. Use spread (`items = [...items, newItem]`) or reassignment after mutation.
- **Svelte 5 (runes):** use `$state()` for reactive variables, `$derived()` for computed values, `$effect()` for side effects. Do not mix runes with legacy `$:` syntax in the same component.
- **Reactive declarations (`$:`):** never put variable assignments that reference other reactive variables inside `$:` blocks. Svelte tracks all read variables as dependencies, causing unwanted re-triggers. Call a function instead: `$: if (condition) initValues();`.
- **Component structure:** keep components under 150 lines. Extract logic into separate `.ts` files or Svelte stores. Props go at the top with `export let` (Svelte 4) or `$props()` (Svelte 5).
- **Stores:** use `writable()` for mutable state, `readable()` for external data sources, `derived()` for computed stores. Subscribe with `$store` auto-subscription in components. Always unsubscribe in non-component code.
- **SvelteKit routing:** use `+page.svelte` for pages, `+layout.svelte` for layouts, `+page.server.ts` for server load functions, `+server.ts` for API endpoints. Load data in `load` functions — never fetch in `onMount` if server-side loading is possible.
- **Form actions:** prefer SvelteKit form actions (`+page.server.ts` with `actions`) over client-side fetch for form submissions. Use `enhance` for progressive enhancement.
- **Lifecycle:** `onMount` for browser-only code (DOM access, subscriptions). Return a cleanup function from `onMount` or use `onDestroy`. `onMount` does not run during SSR.
- **Events:** use `on:event` handlers in Svelte 4, callback props or `$host()` in Svelte 5. Dispatch custom events with `createEventDispatcher` (Svelte 4) or callback props (Svelte 5).
- **Bindings:** use `bind:value` for form inputs. Avoid two-way binding on component props except for simple form-like wrappers — prefer events for parent-child communication.
- **CSS:** styles are scoped by default. Use `:global()` sparingly. Prefer CSS custom properties (`--color`) passed via `style:--color={value}` for theme customization.
- **Transitions and animations:** use built-in `transition:`, `in:`, `out:` directives. Keep animations under 300ms for UI responsiveness.
- **TypeScript:** use `<script lang="ts">`. Type all props, store values, and event payloads. Use `ComponentProps<Component>` to extract prop types from existing components.
- **Testing:** use `@testing-library/svelte` for component tests. Test user interactions and rendered output, not internal state.
