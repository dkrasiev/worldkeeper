import type { WorldData } from "../api";
import { useFormat } from "../format";
import { useI18n } from "../i18n";
import { CopyText } from "./CopyText";
import { Field } from "./ui";

// WorldInfoPanel shows what was read from a world: the live folder or a save.
export function WorldInfoPanel({ w, title }: { w: WorldData; title?: string }) {
  const { t } = useI18n();
  const f = useFormat();
  const pos = w.player?.pos?.map((n) => Math.floor(n)).join(", ");
  const rules = Object.entries(w.gameRules).sort(([a], [b]) => a.localeCompare(b));
  return (
    <section className="panel">
      <h2>{title ?? t("info.world")}</h2>
      <dl className="fields">
        <Field label={t("info.lastPlayed")}>
          {f.dateTime(w.lastPlayed)} <span className="muted">({f.relative(w.lastPlayed)})</span>
        </Field>
        <Field label={t("info.difficulty")}>
          {f.label("difficulty", w.difficulty)}
          {w.difficultyLocked && ` ${t("info.locked")}`}
        </Field>
        <Field label={t("info.cheats")}>{w.cheats ? t("common.on") : t("common.off")}</Field>
        <Field label={t("info.seed")}>{w.seed ? <CopyText text={w.seed} /> : "—"}</Field>
        <Field label={t("info.day")}>{f.num(w.day)}</Field>
        <Field label={t("info.weather")}>{f.label("weather", w.weather)}</Field>
        {w.spawn && <Field label={t("info.spawn")}>{w.spawn.join(", ")}</Field>}
        <Field label={t("info.size")}>{f.bytes(w.sizeBytes)}</Field>
        {w.brands.length > 0 && <Field label={t("info.loaders")}>{w.brands.join(", ")}</Field>}
      </dl>

      {w.player && (
        <>
          <h3>{t("info.player")}</h3>
          <dl className="fields">
            <Field label={t("info.dimension")}>{f.dimension(w.player.dimension)}</Field>
            {pos && <Field label={t("info.position")}>{pos}</Field>}
            <Field label={t("info.health")}>{f.num(w.player.health / 2)} ❤</Field>
            <Field label={t("info.food")}>{f.num(w.player.food / 2)} 🍗</Field>
            <Field label={t("info.level")}>{w.player.xpLevel}</Field>
          </dl>
        </>
      )}

      {w.stats && (
        <>
          <h3>{t("info.stats")}</h3>
          <dl className="fields">
            <Field label={t("info.playTime")}>{f.duration(w.stats.playTimeSeconds)}</Field>
            <Field label={t("info.deaths")}>{f.num(w.stats.deaths)}</Field>
            <Field label={t("info.mobKills")}>{f.num(w.stats.mobKills)}</Field>
            <Field label={t("info.distance")}>{f.km(w.stats.distanceMeters)}</Field>
            <Field label={t("info.jumps")}>{f.num(w.stats.jumps)}</Field>
            <Field label={t("info.advancements")}>{w.advancements}</Field>
          </dl>
        </>
      )}

      <h3>{t("info.dimensions")}</h3>
      <dl className="fields">
        {w.dimensions.map((d) => (
          <Field key={d.id} label={f.dimension(d.id)}>
            {t("info.regions", { count: d.regionFiles })}{" "}
            <span className="muted">({t("info.area", { value: Math.round(d.regionFiles * 0.262144 * 100) / 100 })})</span>
          </Field>
        ))}
      </dl>

      {(w.dataPacks.enabled.length > 0 || rules.length > 0) && (
        <>
          <details>
            <summary>{t("info.dataPacks", { count: w.dataPacks.enabled.length })}</summary>
            <ul className="plain">
              {w.dataPacks.enabled.map((p) => (
                <li key={p} className="mono small">
                  {p}
                </li>
              ))}
            </ul>
          </details>
          <details>
            <summary>{t("info.gameRules", { count: rules.length })}</summary>
            <dl className="fields">
              {rules.map(([k, v]) => (
                <Field key={k} label={k}>
                  <span className="mono">{v}</span>
                </Field>
              ))}
            </dl>
          </details>
        </>
      )}
    </section>
  );
}
