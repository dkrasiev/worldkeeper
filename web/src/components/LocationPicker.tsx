import { useState } from "react";
import { api } from "../api";
import { useLoad } from "../hooks";
import { useI18n } from "../i18n";

const OTHER = "\u0000other"; // the select value for typing a path; never a real path

// Paths from the server and typed ones compare without trailing separators
// and case, which is how Windows and macOS see folders.
const samePath = (a?: string, b?: string) =>
  !!a && !!b && a.replace(/[\\/]+$/, "").toLowerCase() === b.replace(/[\\/]+$/, "").toLowerCase();

// LocationPicker picks a saves folder: one Worldkeeper found (the launchers,
// each launcher instance, the extra folders from Settings) or any folder
// typed in.
export function LocationPicker({
  label,
  value,
  onChange,
  exclude,
  auto,
}: {
  label: string;
  value: string;
  onChange: (saves: string) => void;
  /** The world's own folder, which is no choice for a move. */
  exclude?: string;
  /** With a label, "" is a choice: let Worldkeeper decide. */
  auto?: string;
}) {
  const { t } = useI18n();
  const { data } = useLoad(api.locations, []);
  const locations = (data ?? []).filter((l) => !samePath(l.saves, exclude));
  const [other, setOther] = useState(false);
  const listed = locations.some((l) => samePath(l.saves, value));
  const typing = other || (value !== "" && data !== undefined && !listed);
  const selected = typing ? OTHER : (locations.find((l) => samePath(l.saves, value))?.saves ?? "");

  return (
    <label>
      {label}
      <select
        value={selected}
        onChange={(e) => {
          const v = e.target.value;
          setOther(v === OTHER);
          onChange(v === OTHER ? "" : v);
        }}
      >
        {auto !== undefined ? (
          <option value="">{auto}</option>
        ) : (
          <option value="" disabled>
            {t("location.choose")}
          </option>
        )}
        {locations.map((l) => (
          <option key={l.saves} value={l.saves}>
            {l.label} · {l.saves}
          </option>
        ))}
        <option value={OTHER}>{t("location.other")}</option>
      </select>
      {typing && (
        <input
          className="mono"
          autoFocus
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("location.placeholder")}
        />
      )}
      <span className="hint">{t("location.hint")}</span>
    </label>
  );
}
