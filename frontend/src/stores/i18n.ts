import { writable, derived } from 'svelte/store';

export type Language = 'de' | 'en' | 'it' | 'es' | 'fr';

export const LANGUAGES: { code: Language; label: string }[] = [
  { code: 'de', label: 'Deutsch' },
  { code: 'en', label: 'English' },
  { code: 'it', label: 'Italiano' },
  { code: 'es', label: 'Español' },
  { code: 'fr', label: 'Français' },
];

export type TranslationKeys = typeof import('../i18n/de').default;

const translations: Record<Language, () => Promise<{ default: TranslationKeys }>> = {
  de: () => import('../i18n/de'),
  en: () => import('../i18n/en'),
  it: () => import('../i18n/it'),
  es: () => import('../i18n/es'),
  fr: () => import('../i18n/fr'),
};

export const currentLang = writable<Language>('de');

// Flat translation dictionary for the active language
let currentDict: Record<string, string> = {};

// A version counter to trigger reactivity when dict changes
const dictVersion = writable(0);

/** Load translations for the given language. */
export async function setLanguage(lang: Language) {
  const mod = await translations[lang]();
  currentDict = flatten(mod.default);
  currentLang.set(lang);
  dictVersion.update(v => v + 1);
}

/** Reactive translate function — use as $t('key') in components. */
export const t = derived(
  [currentLang, dictVersion],
  () => {
    return (key: string, params?: Record<string, string | number>): string => {
      let val = currentDict[key] || key;
      if (params) {
        for (const [k, v] of Object.entries(params)) {
          val = val.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
        }
      }
      return val;
    };
  }
);

/** Flatten nested object to dot-notation keys. */
function flatten(obj: Record<string, unknown>, prefix = ''): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (typeof value === 'string') {
      result[fullKey] = value;
    } else if (typeof value === 'object' && value !== null) {
      Object.assign(result, flatten(value as Record<string, unknown>, fullKey));
    }
  }
  return result;
}

/** Initialise with a language (call once on app start). */
export async function initI18n(lang: Language) {
  await setLanguage(lang);
}
