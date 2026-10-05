import { useMemo } from "react";
import { useI18n } from "./i18n";

const hasDate = (iso?: string) => !!iso && new Date(iso).getFullYear() > 2000;

const steps: [number, Intl.RelativeTimeFormatUnit][] = [
  [60, "second"],
  [60, "minute"],
  [24, "hour"],
  [30, "day"],
  [12, "month"],
  [Infinity, "year"],
];

const byteUnits = ["byte", "kilobyte", "megabyte", "gigabyte", "terabyte"];

// useFormat returns formatters bound to the current UI language.
export function useFormat() {
  const { locale, tryT } = useI18n();
  return useMemo(() => {
    const dt = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });
    const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    const nf = new Intl.NumberFormat(locale);

    return {
      dateTime: (iso?: string) => (hasDate(iso) ? dt.format(new Date(iso!)) : "—"),

      relative: (iso?: string) => {
        if (!hasDate(iso)) return "—";
        let value = (Date.now() - new Date(iso!).getTime()) / 1000;
        for (const [size, unit] of steps) {
          if (Math.abs(value) < size) return rtf.format(-Math.round(value), unit);
          value /= size;
        }
        return "";
      },

      bytes: (n: number) => {
        let i = 0;
        while (n >= 1024 && i < byteUnits.length - 1) {
          n /= 1024;
          i++;
        }
        return new Intl.NumberFormat(locale, {
          style: "unit",
          unit: byteUnits[i],
          unitDisplay: "short",
          maximumFractionDigits: n < 10 && i > 0 ? 1 : 0,
        }).format(n);
      },

      duration: (seconds: number) => {
        const unit = (u: string, v: number) =>
          new Intl.NumberFormat(locale, { style: "unit", unit: u, unitDisplay: "short" }).format(v);
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        return h > 0 ? `${unit("hour", h)} ${unit("minute", m)}` : unit("minute", m);
      },

      km: (meters: number) =>
        new Intl.NumberFormat(locale, { style: "unit", unit: "kilometer", maximumFractionDigits: 1 }).format(
          meters / 1000,
        ),

      num: (n: number) => nf.format(n),

      /** Known dimensions by name; modded ones keep their id. */
      dimension: (id: string) => tryT(`dim.${id}`) ?? id,

      /** Enum-like values from the world (difficulty, weather) via "<prefix>.<value>". */
      label: (prefix: string, value: string) => tryT(`${prefix}.${value}`) ?? (value || "—"),
    };
  }, [locale, tryT]);
}
