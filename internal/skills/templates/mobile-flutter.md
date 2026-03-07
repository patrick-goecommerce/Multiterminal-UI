You are working on a Flutter project. Follow these principles:

- Follow a clean architecture pattern: separate presentation, domain, and data layers. Keep business logic out of widgets.
- Use Riverpod or BLoC for state management. Avoid `setState` in anything beyond simple, single-widget local state.
- Prefer `StatelessWidget` over `StatefulWidget` when possible. Use state management solutions to handle reactive state.
- Break large widgets into smaller, composable widgets. Extract widget methods into separate widget classes for better rebuild performance.
- Use `const` constructors wherever possible. Mark widgets as `const` to prevent unnecessary rebuilds.
- Use `ListView.builder` and `GridView.builder` for long lists. Never use `Column` with `SingleChildScrollView` for dynamic-length lists.
- Handle navigation with GoRouter or auto_route for declarative, type-safe routing with deep link support.
- Use platform channels for native functionality not covered by packages. Keep channel method signatures versioned and documented.
- Implement proper error handling: use `Either` types or sealed classes for result types, display user-friendly error messages.
- Use `freezed` or `json_serializable` for immutable data classes and JSON serialization. Avoid manual `fromJson`/`toJson` boilerplate.
- Test widgets with `testWidgets`, business logic with unit tests, and use `integration_test` for end-to-end flows.
- Follow Material 3 design guidelines by default. Use `ThemeData` and `ColorScheme` for consistent theming across the app.
- Use `cached_network_image` for image loading and caching. Serve appropriately sized images from your backend.
- Manage dependencies with `pubspec.yaml`. Pin versions in production apps. Run `dart fix --apply` and `flutter analyze` in CI.
- Use Dart's null safety fully. Avoid `!` (bang operator) unless you are absolutely certain the value is non-null. Prefer null-aware operators and pattern matching.
