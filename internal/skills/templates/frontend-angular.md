You are working on an Angular project. Follow these principles:

- **Standalone components** are the default (Angular 14+). Use `standalone: true` and import dependencies directly. Only use NgModules when the codebase requires them.
- **Component design:** keep components focused on presentation. Extract business logic into services. Use `OnPush` change detection for all components to minimize unnecessary re-renders.
- **RxJS discipline:** prefer declarative streams over imperative subscriptions. Use `async` pipe in templates instead of manual `.subscribe()`. When subscribing manually, always unsubscribe — use `takeUntilDestroyed()` (Angular 16+) or `DestroyRef`.
- **RxJS operators:** use `switchMap` for cancellable requests (search, navigation), `concatMap` for ordered sequential operations, `mergeMap` for parallel operations, `exhaustMap` for ignoring new emissions while processing.
- **Dependency injection:** provide services at the narrowest possible scope. Use `providedIn: 'root'` for singletons, component-level `providers` for scoped instances. Use `inject()` function over constructor injection in modern Angular.
- **Reactive forms** over template-driven forms for anything beyond trivial inputs. Define form groups with `FormBuilder`, use validators from `Validators`, and create custom validators as pure functions.
- **Signals (Angular 16+):** use `signal()` for local reactive state, `computed()` for derived values, `effect()` for side effects. Prefer signals over BehaviorSubject for new component state.
- **Routing:** use lazy loading (`loadComponent` / `loadChildren`) for all feature routes. Use route guards as functions (`CanActivateFn`), not classes. Resolve data in route resolvers or `OnInit`.
- **HTTP:** use `HttpClient` with typed responses. Centralize API calls in services. Use interceptors (functional interceptors in Angular 15+) for auth tokens, error handling, and caching.
- **Error handling:** implement a global `ErrorHandler`. Use HTTP interceptors to catch and transform API errors. Show user-friendly error messages — never expose raw error objects.
- **Testing:** use `TestBed` with minimal module configuration. Test components with `ComponentFixture`, services with plain unit tests. Mock dependencies with `jasmine.createSpyObj` or `jest.fn()`. Prefer `spectator` or `testing-library` for less boilerplate.
- **File structure:** one component/service/pipe per file. Follow the naming convention: `feature-name.component.ts`, `feature-name.service.ts`. Group by feature, not by type.
- **Templates:** keep templates under 50 lines. Use `@if`/`@for` control flow (Angular 17+) over `*ngIf`/`*ngFor`. Extract complex template logic into computed signals or component methods.
- **Performance:** use `trackBy` with `@for` loops. Defer heavy components with `@defer`. Lazy-load images with `NgOptimizedImage`.
