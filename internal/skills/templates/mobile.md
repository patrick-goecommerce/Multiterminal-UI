You are working on a mobile application (iOS, Android, React Native, Flutter). Follow these principles:

- **Platform conventions:** respect platform design guidelines — Material Design for Android, Human Interface Guidelines for iOS. Users expect native-feeling interactions, navigation patterns, and gestures.
- **Navigation:** use platform-standard navigation patterns — stack navigation for drill-down, tab bar for top-level sections, drawer for secondary navigation. Handle deep links and universal links from day one.
- **Performance:** target 60fps for all animations and scrolling. Use virtualized/recycled lists for long content. Profile on real devices — simulators don't reflect actual performance. Monitor app startup time (cold start < 2s).
- **Offline-first:** design for intermittent connectivity. Cache data locally (SQLite, MMKV, Realm). Queue mutations for sync when online. Show stale data with a freshness indicator rather than empty screens.
- **State management:** use unidirectional data flow. React Native: Zustand, Redux Toolkit, or React Query. Flutter: Riverpod or BLoC. SwiftUI: @Observable. Jetpack Compose: ViewModel + StateFlow.
- **Networking:** use retry with exponential backoff for network calls. Set reasonable timeouts (10-30s). Cancel in-flight requests when the user navigates away. Handle all HTTP error codes gracefully.
- **Push notifications:** request permission at a contextually appropriate moment, not on first launch. Handle foreground, background, and killed-state notifications differently. Use silent push for data sync.
- **App lifecycle:** handle backgrounding, foregrounding, and termination gracefully. Save state before backgrounding. Restore state on resume. Handle low-memory warnings by releasing caches.
- **Security:** store sensitive data in Keychain (iOS) or EncryptedSharedPreferences (Android). Use certificate pinning for API calls. Obfuscate release builds. Never hardcode API keys or secrets in the bundle.
- **Testing:** unit-test business logic. Widget/UI-test critical user flows. Run e2e tests on real devices via CI (Detox, XCUITest, Espresso). Test on the oldest supported OS version and smallest screen size.
- **App Store compliance:** follow App Store Review Guidelines and Google Play policies. Handle in-app purchases correctly. Provide privacy labels and data handling disclosures. Test the release build, not just debug.
- **Images and assets:** serve appropriately sized images. Use WebP/AVIF formats. Implement placeholder shimmer during loading. Use @2x/@3x assets for iOS, density buckets for Android.
- **Accessibility:** support Dynamic Type (iOS) and font scaling (Android). Add content descriptions to images and icons. Ensure all interactive elements are reachable via TalkBack/VoiceOver. Test with accessibility tools.
- **Release management:** use semantic versioning. Implement staged rollouts (1% → 10% → 100%). Use OTA updates for JS bundles (CodePush, Expo Updates). Monitor crash-free rate and ANR rate post-release.
