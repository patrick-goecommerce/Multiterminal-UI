You are working on a Vue 3 project. Follow these principles:

- **Composition API with `<script setup>`.** Prefer `<script setup lang="ts">` for all new components. Avoid the Options API unless the codebase already uses it consistently.
- **Reactivity:** use `ref()` for primitives, `reactive()` for objects. Access ref values with `.value` in script, but not in templates. Use `computed()` for derived state — never recalculate in templates.
- **Composables:** extract reusable logic into `useXxx` functions in a `composables/` directory. Composables should return refs and functions, accepting refs as arguments for reactivity.
- **Template refs:** type them as `ref<HTMLElement | null>(null)` and access after `onMounted`. Use `defineExpose` only when parent components genuinely need child internals.
- **Props and emits:** use `defineProps<T>()` and `defineEmits<T>()` with TypeScript generics. Always define prop types explicitly; avoid `Object` or `Array` without generics.
- **State management with Pinia:** define stores using the setup syntax (`defineStore('id', () => { ... })`). Keep stores focused — one per domain concern. Access stores via `useXxxStore()` composables.
- **Watchers:** prefer `watchEffect` for side effects that should re-run when any dependency changes. Use `watch` with explicit sources when you need the old value or want to control timing (`{ immediate: true, flush: 'post' }`).
- **Component design:** keep templates under 80 lines. Extract repeated template blocks into child components. Use `<slot>` for content projection and named slots for layout composition.
- **Nuxt conventions:** use `pages/` for file-based routing, `server/api/` for API routes, `composables/` for auto-imported composables. Fetch data with `useFetch` or `useAsyncData` — never in `onMounted` if it can run server-side.
- **Provide/inject:** use typed injection keys (`InjectionKey<T>`). Prefer Pinia over provide/inject for state that crosses multiple component levels.
- **Lifecycle:** `onMounted` for DOM access, `onUnmounted` for cleanup (event listeners, timers, subscriptions). Never do async work in the synchronous setup flow without `await` inside `<Suspense>`.
- **Testing:** use `@vue/test-utils` with `mount` or `shallowMount`. Test component output and emitted events. Stub child components only when their rendering is expensive or irrelevant to the test.
- **TypeScript:** enable strict mode. Type all props, emits, refs, and composable return values. Use `PropType<T>` only in Options API — prefer generic `defineProps` in `<script setup>`.
