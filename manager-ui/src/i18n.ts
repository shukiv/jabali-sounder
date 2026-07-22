// i18n bootstrap. English is the SOURCE language: `src/locales/en/common.json`
// is the only catalog edited by hand in this repo. Every other locale is written
// by translators through Weblate and lands here as a PR — never hand-edit them.
//
// Locale codes match the directory names, which are also what Weblate uses as
// the language code (filemask `manager-ui/src/locales/*/common.json`).
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import type { Locale as AntdLocale } from "antd/es/locale";
import enUS from "antd/locale/en_US";
import heIL from "antd/locale/he_IL";
import deDE from "antd/locale/de_DE";
import esES from "antd/locale/es_ES";
import frFR from "antd/locale/fr_FR";
import itIT from "antd/locale/it_IT";
import jaJP from "antd/locale/ja_JP";
import ukUA from "antd/locale/uk_UA";
import zhCN from "antd/locale/zh_CN";
import ptBR from "antd/locale/pt_BR";
import ruRU from "antd/locale/ru_RU";
import trTR from "antd/locale/tr_TR";
import arEG from "antd/locale/ar_EG";

import enCommon from "./locales/en/common.json";
import heCommon from "./locales/he/common.json";
import deCommon from "./locales/de/common.json";
import esCommon from "./locales/es/common.json";
import frCommon from "./locales/fr/common.json";
import itCommon from "./locales/it/common.json";
import jaCommon from "./locales/ja/common.json";
import ukCommon from "./locales/uk/common.json";
import zhHansCommon from "./locales/zh_Hans/common.json";
import ptBRCommon from "./locales/pt_BR/common.json";
import ruCommon from "./locales/ru/common.json";
import trCommon from "./locales/tr/common.json";
import arCommon from "./locales/ar/common.json";

export const DEFAULT_LNG = "en";

/** Locales Sounder ships. Keep in sync with src/locales/*. */
export const SUPPORTED = [
  "en", "he", "de", "es", "fr", "it", "ja", "uk", "zh_Hans", "pt_BR", "ru", "tr", "ar",
] as const;
export type SupportedLng = (typeof SUPPORTED)[number];

/**
 * Language names written in the language itself: someone who cannot read the
 * current UI language still recognises their own. Intl.DisplayNames would render
 * these in the *active* locale, so the entry would stop being self-identifying.
 */
export const LANGUAGE_LABELS: Record<SupportedLng, string> = {
  en: "English",
  he: "עברית",
  de: "Deutsch",
  es: "Español",
  fr: "Français",
  it: "Italiano",
  ja: "日本語",
  uk: "Українська",
  zh_Hans: "中文（简体）",
  pt_BR: "Português (Brasil)",
  ru: "Русский",
  tr: "Türkçe",
  ar: "العربية",
};

/** Right-to-left scripts. Drives AntD's `direction` + the <html dir> attribute. */
const RTL = new Set<string>(["he", "ar", "fa", "ur"]);

const ANTD_LOCALES: Record<SupportedLng, AntdLocale> = {
  en: enUS, he: heIL, de: deDE, es: esES, fr: frFR, it: itIT, ja: jaJP,
  uk: ukUA, zh_Hans: zhCN, pt_BR: ptBR, ru: ruRU, tr: trTR, ar: arEG,
};

const STORAGE_KEY = "jabali-sounder-lng";

/** Map a browser/stored tag onto a locale we actually ship ("zh-CN" -> zh_Hans). */
function normalise(raw: string | null | undefined): SupportedLng {
  if (!raw) return DEFAULT_LNG;
  const lower = raw.replace(/_/g, "-").toLowerCase();
  const exact = SUPPORTED.find((l) => l.replace(/_/g, "-").toLowerCase() === lower);
  if (exact) return exact;
  const base = lower.split("-")[0];
  const byBase = SUPPORTED.find((l) => l.replace(/_/g, "-").toLowerCase().split("-")[0] === base);
  return byBase ?? DEFAULT_LNG;
}

/** Explicit user choice (localStorage) → browser preference → English. */
function detect(): SupportedLng {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored) return normalise(stored);
  } catch {
    // localStorage can throw in private mode — fall through to the browser.
  }
  return normalise(typeof navigator !== "undefined" ? navigator.language : null);
}

export function isRTL(lng: string = i18n.resolvedLanguage ?? DEFAULT_LNG): boolean {
  return RTL.has(normalise(lng));
}

export function antdLocale(lng: string = i18n.resolvedLanguage ?? DEFAULT_LNG): AntdLocale {
  return ANTD_LOCALES[normalise(lng)] ?? enUS;
}

/** Apply <html lang>/<html dir> for the active language. */
function applyDocumentLocale(lng: string): void {
  const l = normalise(lng);
  if (typeof document !== "undefined") {
    document.documentElement.setAttribute("lang", l.replace(/_/g, "-"));
    document.documentElement.setAttribute("dir", isRTL(l) ? "rtl" : "ltr");
  }
}

/** Switch language + remember the choice. */
export async function setLanguage(lng: string): Promise<void> {
  const l = normalise(lng);
  try {
    window.localStorage.setItem(STORAGE_KEY, l);
  } catch {
    // Non-fatal: the language still applies for this session.
  }
  await i18n.changeLanguage(l);
}

void i18n.use(initReactI18next).init({
  lng: detect(),
  fallbackLng: DEFAULT_LNG,
  supportedLngs: SUPPORTED as unknown as string[],
  nonExplicitSupportedLngs: true,
  defaultNS: "common",
  ns: ["common"],
  resources: {
    en: { common: enCommon },
    he: { common: heCommon },
    de: { common: deCommon },
    es: { common: esCommon },
    fr: { common: frCommon },
    it: { common: itCommon },
    ja: { common: jaCommon },
    uk: { common: ukCommon },
    zh_Hans: { common: zhHansCommon },
    pt_BR: { common: ptBRCommon },
    ru: { common: ruCommon },
    tr: { common: trCommon },
    ar: { common: arCommon },
  },
  interpolation: { escapeValue: false },
  // A key with no translation yet renders the English source, never a blank
  // label — important while Weblate catalogs are still filling up.
  returnEmptyString: false,
});

applyDocumentLocale(i18n.resolvedLanguage ?? DEFAULT_LNG);
i18n.on("languageChanged", applyDocumentLocale);

export default i18n;
