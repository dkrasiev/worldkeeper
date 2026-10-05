const hasDate = (iso?: string) => !!iso && new Date(iso).getFullYear() > 2000;

export function dateTime(iso?: string): string {
  if (!hasDate(iso)) return "—";
  return new Date(iso!).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

export function relative(iso?: string): string {
  if (!hasDate(iso)) return "never";
  const sec = Math.round((Date.now() - new Date(iso!).getTime()) / 1000);
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
  const steps: [number, Intl.RelativeTimeFormatUnit][] = [
    [60, "second"],
    [60, "minute"],
    [24, "hour"],
    [30, "day"],
    [12, "month"],
    [Infinity, "year"],
  ];
  let value = sec;
  for (const [size, unit] of steps) {
    if (Math.abs(value) < size) return rtf.format(-Math.round(value), unit);
    value /= size;
  }
  return "";
}

export function bytes(n: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n < 10 && i > 0 ? 1 : 0)} ${units[i]}`;
}

export function duration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return h > 0 ? `${h} h ${m} min` : `${m} min`;
}

export const num = (n: number) => n.toLocaleString();

export function dimensionName(id: string): string {
  const known: Record<string, string> = {
    "minecraft:overworld": "Overworld",
    "minecraft:the_nether": "Nether",
    "minecraft:the_end": "The End",
  };
  return known[id] ?? id;
}

export const capitalize = (s: string) => (s ? s[0].toUpperCase() + s.slice(1) : "—");
