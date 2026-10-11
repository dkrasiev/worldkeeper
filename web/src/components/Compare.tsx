import { useEffect, useMemo, useState } from "react";
import { api, type WorldData, type WorldInfo } from "../api";
import { compareWorlds, unchanged, type Progress } from "../compare";
import { useFormat } from "../format";
import { useI18n } from "../i18n";
import { Field } from "./ui";

// Lists of advancements longer than this start collapsed.
const OPEN_LIST = 10;

// SaveComparison shows what changed in the live world since a save: the save
// page highlights it above the save's own contents.
export function SaveComparison({
  id,
  snapId,
  save,
  now,
}: {
  id: string;
  snapId: string;
  save: WorldData;
  now: WorldInfo;
}) {
  const { t } = useI18n();
  const f = useFormat();
  const [progress, setProgress] = useState<[Progress, Progress]>();

  // The live world's progress changes whenever the game saves, which also
  // moves lastPlayed. The previous lists stay up while the new ones load.
  useEffect(() => {
    let cancelled = false;
    Promise.all([api.snapshotAdvancements(id, snapId), api.advancements(id)]).then(
      (p) => !cancelled && setProgress(p),
      () => !cancelled && setProgress(undefined), // the counts still compare
    );
    return () => {
      cancelled = true;
    };
  }, [id, snapId, now.lastPlayed]);

  const d = useMemo(() => compareWorlds(save, now, progress?.[0], progress?.[1]), [save, now, progress]);

  const delta = (n: number, text: string) =>
    n !== 0 && <span className={`delta ${n > 0 ? "up" : "down"}`}>{(n > 0 ? "+" : "−") + text}</span>;
  const count = (from: number, to: number) => (
    <>
      {f.num(from)} → {f.num(to)} {delta(to - from, f.num(Math.abs(to - from)))}
    </>
  );

  return (
    <section className="panel compare">
      <h2>{t(d.saveIsNewer ? "compare.titleNewer" : "compare.title")}</h2>
      {d.saveIsNewer && <p className="muted small">{t("compare.newerHint")}</p>}
      {now.inUse && <p className="muted small">{t("compare.inUse")}</p>}

      {unchanged(d) ? (
        <p className="muted">{t("compare.same")}</p>
      ) : (
        <>
          <dl className="fields">
            {d.version && (
              <Field label={t("compare.version")}>
                {d.version.from} → {d.version.to}
              </Field>
            )}
            {d.playTimeSeconds && (
              <Field label={t("info.playTime")}>
                {f.duration(d.playTimeSeconds.from)} → {f.duration(d.playTimeSeconds.to)}{" "}
                {delta(
                  d.playTimeSeconds.to - d.playTimeSeconds.from,
                  f.duration(Math.abs(d.playTimeSeconds.to - d.playTimeSeconds.from)),
                )}
              </Field>
            )}
            {d.advancements && (
              <Field label={t("info.advancements")}>{count(d.advancements.from, d.advancements.to)}</Field>
            )}
            {d.regions.map((r) => (
              <Field key={r.id} label={t("compare.regions", { dimension: f.dimension(r.id) })}>
                {count(r.from, r.to)}
              </Field>
            ))}
          </dl>

          {d.advancements && d.advancements.gained.length > 0 && (
            <AdvancementList
              title={t("compare.gained", { count: d.advancements.gained.length })}
              ids={d.advancements.gained}
            />
          )}
          {d.advancements && d.advancements.lost.length > 0 && (
            <AdvancementList
              title={t("compare.lost", { count: d.advancements.lost.length })}
              ids={d.advancements.lost}
            />
          )}

          {d.gameRules.length > 0 && (
            <>
              <h3>{t("compare.gameRules")}</h3>
              <dl className="fields">
                {d.gameRules.map((r) => (
                  <Field key={r.rule} label={r.rule}>
                    <span className="mono">
                      {r.from} → {r.to}
                    </span>
                  </Field>
                ))}
              </dl>
            </>
          )}
        </>
      )}
    </section>
  );
}

// Advancements have no translated names here; the id without the
// minecraft: namespace reads well enough ("story/mine_stone").
function AdvancementList({ title, ids }: { title: string; ids: string[] }) {
  return (
    <details open={ids.length <= OPEN_LIST}>
      <summary>{title}</summary>
      <ul className="plain">
        {ids.map((id) => (
          <li key={id} className="mono small" title={id}>
            {id.replace(/^minecraft:/, "")}
          </li>
        ))}
      </ul>
    </details>
  );
}
