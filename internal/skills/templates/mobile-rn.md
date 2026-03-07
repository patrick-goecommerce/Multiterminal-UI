You are working on a React Native project. Follow these principles:

- Use Expo for new projects unless you need custom native modules not supported by Expo. Use `expo prebuild` (CNG) to eject only when necessary.
- Structure navigation with React Navigation. Use typed route params with TypeScript. Prefer stack + tab navigators for standard app flows.
- Use `FlatList` or `FlashList` for long lists, never `ScrollView` with `.map()`. Implement `keyExtractor`, `getItemLayout`, and `windowSize` tuning.
- Avoid anonymous functions in `renderItem` and event handlers passed to list items. Use `useCallback` to prevent unnecessary re-renders.
- Handle platform differences with `Platform.select()` or `.ios.tsx`/`.android.tsx` file extensions. Keep platform-specific code isolated.
- Use `react-native-mmkv` or `expo-secure-store` for local storage. Avoid `AsyncStorage` for sensitive data.
- Implement proper error boundaries and crash reporting (Sentry, Bugsnag). React Native crashes can be silent in production.
- Use `react-native-reanimated` and `react-native-gesture-handler` for animations and gestures. Run animations on the UI thread, not the JS thread.
- Handle the keyboard properly: use `KeyboardAvoidingView`, `keyboard` behavior per platform, and test on both iOS and Android.
- Test on real devices, not just simulators. Performance and behavior differ significantly, especially for animations and gestures.
- Use `expo-updates` or CodePush for OTA updates of JS bundles. Reserve full app store releases for native code changes.
- Manage environment-specific configs with `expo-constants` or `react-native-config`. Never hardcode API URLs or keys.
- Optimize images: use `expo-image` or `react-native-fast-image` for caching, serve appropriately sized images, use WebP format.
- Handle deep linking and universal links from the start. Configure URL schemes and associated domains early.
- Use Hermes as the JavaScript engine (default in modern RN). Enable the new architecture (Fabric + TurboModules) for performance-critical apps.
