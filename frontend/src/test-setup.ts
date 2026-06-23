// Global Vitest setup. Component tests render against German UI strings, so the
// i18n dictionary must be loaded before any component mounts — otherwise $t()
// returns the raw key (e.g. "crash.title") instead of the translation.
import { initI18n } from './stores/i18n';

await initI18n('de');
