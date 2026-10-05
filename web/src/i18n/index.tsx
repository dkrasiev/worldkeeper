import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { ApiError } from "../api";
import { en } from "./en";
import { ru } from "./ru";
import type { Dict, MessageKey, Params, Plural } from "./types";

export type { MessageKey } from "./types";

// To add a language: create <code>.ts typed as Dict and register it here.
const dictionaries = { en, ru } satisfies Record<string, Dict>;
export type Locale = keyof typeof dictionaries;
export const LOCALES: { code: Locale; name: string }[] = [
  { code: "en", name: "English" },
  { code: "ru", name: "Русский" },
];

const STORAGE_KEY = "worldkeeper.locale";

function isLocale(v: string | null | undefined): v is Locale {
  return !!v && v in dictionaries;
}

// A saved choice wins; otherwise the first browser language we support.
function detectLocale(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (isLocale(saved)) return saved;
  } catch {
    /* storage blocked */
  }
  for (const tag of navigator.languages ?? [navigator.language]) {
    const base = tag.toLowerCase().split("-")[0];
    if (isLocale(base)) return base;
  }
  return "en";
}

export interface I18n {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: (key: MessageKey, params?: Params) => string;
  /** Translate a key built at runtime (server codes); undefined if unknown. */
  tryT: (key: string, params?: Params) => string | undefined;
  /** User-facing text for any error, translated when the server sent a known key. */
  errorText: (e: unknown) => string;
}

function makeI18n(locale: Locale, setLocale: (l: Locale) => void): I18n {
  const dict: Dict = dictionaries[locale];
  const plurals = new Intl.PluralRules(locale);
  const numbers = new Intl.NumberFormat(locale);

  const fill = (template: string, params?: Params) =>
    template.replace(/\{(\w+)\}/g, (m, name: string) => {
      const v = params?.[name];
      if (v === undefined) return m;
      return typeof v === "number" ? numbers.format(v) : v;
    });

  const render = (entry: string | Plural, params?: Params) => {
    if (typeof entry === "string") return fill(entry, params);
    const count = Number(params?.count ?? 0);
    const form = plurals.select(count) as keyof Plural;
    return fill(entry[form] ?? entry.other, params);
  };

  const tryT = (key: string, params?: Params) => {
    const entry = (dict as Record<string, string | Plural>)[key] ?? (en as Record<string, string | Plural>)[key];
    return entry === undefined ? undefined : render(entry, params);
  };

  return {
    locale,
    setLocale,
    t: (key, params) => render(dict[key] ?? en[key], params),
    tryT,
    errorText: (e) => {
      if (e instanceof ApiError) {
        return (
          (e.key && tryT(`err.${e.key}`, e.params)) || tryT(`err.${e.code}`, e.params) || e.message
        );
      }
      return e instanceof Error ? e.message : String(e);
    },
  };
}

const I18nContext = createContext<I18n | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(detectLocale);

  const value = useMemo(
    () =>
      makeI18n(locale, (l) => {
        try {
          localStorage.setItem(STORAGE_KEY, l);
        } catch {
          /* remembered for this page only */
        }
        setLocaleState(l);
      }),
    [locale],
  );

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18n {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n outside I18nProvider");
  return ctx;
}
