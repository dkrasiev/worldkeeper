import { useState } from "react";
import { api, ApiError, type RestoreMode, type Snapshot, type WorldInfo } from "../api";
import { Field, KindBadge, Modal, ModeBadge, WorldIcon } from "../components/ui";
import { CopyText } from "../components/CopyText";
import { AdvancementsPanel } from "../components/Advancements";
import { useFormat } from "../format";
import { useLoad } from "../hooks";
import { useI18n } from "../i18n";

// Safety snapshots store only what they preceded ("“Label”" or a date);
// labels written by older versions carried an English prefix.
const legacyPrefix = /^(Before loading |before restoring )/;

function useSnapTitle() {
  const { t } = useI18n();
  const f = useFormat();
  return (s: Snapshot) => {
    if (s.kind === "pre-restore" && s.label) return t("snap.before", { name: s.label.replace(legacyPrefix, "") });
    return s.label || f.dateTime(s.createdAt);
  };
}

type Dialog =
  | { type: "save"; force?: boolean }
  | { type: "load"; snap: Snapshot }
  | { type: "delete"; snap: Snapshot };

export function WorldDetail({ id, onBack }: { id: string; onBack: () => void }) {
  const { t, errorText } = useI18n();
  const f = useFormat();
  const title = useSnapTitle();
  const info = useLoad(() => api.world(id), [id], 5000);
  const snaps = useLoad(() => api.snapshots(id), [id], 5000);
  const [dialog, setDialog] = useState<Dialog>();
  const [notice, setNotice] = useState<{ ok: boolean; text: string }>();

  const missing = info.error instanceof ApiError && info.error.status === 404;
  const world = info.data;
  const ref = snaps.data?.world;
  const list = snaps.data?.snapshots ?? [];

  const refresh = () => {
    info.reload();
    snaps.reload();
  };
  const run = async (fn: () => Promise<string>) => {
    try {
      setNotice({ ok: true, text: await fn() });
      setDialog(undefined);
      refresh();
    } catch (e) {
      if (e instanceof ApiError && e.code === "in_use" && dialog?.type === "save") {
        setDialog({ type: "save", force: true });
        return;
      }
      setNotice({ ok: false, text: errorText(e) });
      setDialog(undefined);
    }
  };

  if (info.loading && snaps.loading) return <p className="muted">{t("common.loading")}</p>;

  return (
    <>
      <button className="link" onClick={onBack}>
        {t("detail.back")}
      </button>

      <header className="world-header">
        <WorldIcon id={id} hasIcon={world?.hasIcon} size={96} />
        <div>
          <h1>{world?.name ?? ref?.name ?? id}</h1>
          <div className="world-meta">
            {world && <ModeBadge mode={world.gameMode} hardcore={world.hardcore} />}
            {world?.gameVersion && <span className="badge">{world.gameVersion}</span>}
            {world?.inUse && <span className="status live">{t("status.inGame")}</span>}
            {missing && <span className="status warn">{t("status.notHere")}</span>}
          </div>
          <div className="muted small mono">{world?.path ?? ref?.path}</div>
        </div>
        {world && (
          <button className="primary" onClick={() => setDialog({ type: "save" })}>
            {t("detail.saveNow")}
          </button>
        )}
      </header>

      {notice && (
        <div className={`callout ${notice.ok ? "success" : "error"}`} onClick={() => setNotice(undefined)}>
          {notice.text}
        </div>
      )}
      {info.error && !missing && <div className="callout error">{errorText(info.error)}</div>}

      <div className="columns">
        <section className="panel">
          <h2>{t("detail.saves")}</h2>
          {list.length === 0 && <p className="muted">{t("detail.noSaves")}</p>}
          <ul className="snapshots">
            {list.map((s) => (
              <li key={s.id} className="snapshot">
                <div className="snapshot-main">
                  <div>
                    <KindBadge kind={s.kind} /> <strong>{title(s)}</strong>
                  </div>
                  {s.note && <div className="small">{s.note}</div>}
                  <div className="muted small">
                    {s.label && <>{f.dateTime(s.createdAt)} · </>}
                    {f.bytes(s.sizeBytes)}
                    {s.gameVersion && <> · {s.gameVersion}</>}
                  </div>
                </div>
                <div className="snapshot-actions">
                  <button onClick={() => setDialog({ type: "load", snap: s })}>{t("common.load")}</button>
                  <button className="ghost" title={t("common.delete")} aria-label={t("common.delete")} onClick={() => setDialog({ type: "delete", snap: s })}>
                    ✕
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </section>

        {world && <InfoPanel w={world} />}
      </div>

      {world && <AdvancementsPanel id={id} earned={world.advancements} />}

      {dialog?.type === "save" && (
        <SaveDialog
          force={!!dialog.force}
          onClose={() => setDialog(undefined)}
          onSave={(label, note) =>
            run(async () => {
              const s = await api.save(id, label, note, dialog.force);
              return t("detail.saved", { name: title(s) });
            })
          }
        />
      )}
      {dialog?.type === "load" && (
        <LoadDialog
          title={title(dialog.snap)}
          missing={missing}
          inUse={!!world?.inUse}
          onClose={() => setDialog(undefined)}
          onLoad={(mode) =>
            run(async () => {
              const r = await api.restore(id, dialog.snap.id, mode);
              return t("detail.loadedInto", { path: r.path });
            })
          }
        />
      )}
      {dialog?.type === "delete" && (
        <Modal title={t("detail.deleteTitle")} onClose={() => setDialog(undefined)}>
          <p>{t("detail.deleteBody", { name: title(dialog.snap) })}</p>
          <div className="actions">
            <button onClick={() => setDialog(undefined)}>{t("common.cancel")}</button>
            <button
              className="danger"
              onClick={() =>
                run(async () => {
                  await api.deleteSnapshot(id, dialog.snap.id);
                  return t("detail.deleted");
                })
              }
            >
              {t("common.delete")}
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}

function SaveDialog({
  force,
  onClose,
  onSave,
}: {
  force: boolean;
  onClose: () => void;
  onSave: (label: string, note: string) => void;
}) {
  const { t } = useI18n();
  const [label, setLabel] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <Modal title={t("save.title")} onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setBusy(true);
          onSave(label, note);
        }}
      >
        {force && (
          <div className="callout warn">{t("save.inUseWarning")}</div>
        )}
        <label>
          {t("save.name")}
          <input autoFocus value={label} onChange={(e) => setLabel(e.target.value)} placeholder={t("save.namePlaceholder")} />
        </label>
        <label>
          {t("save.note")}
          <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} placeholder={t("save.optional")} />
        </label>
        <div className="actions">
          <button type="button" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className={force ? "danger" : "primary"} disabled={busy}>
            {busy ? t("save.saving") : force ? t("save.anyway") : t("save.save")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function LoadDialog({
  title,
  missing,
  inUse,
  onClose,
  onLoad,
}: {
  title: string;
  missing: boolean;
  inUse: boolean;
  onClose: () => void;
  onLoad: (mode: RestoreMode) => void;
}) {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const go = (mode: RestoreMode) => {
    setBusy(true);
    onLoad(mode);
  };
  return (
    <Modal title={t("load.title", { name: title })} onClose={onClose}>
      {missing ? (
        <p>{t("load.missingHint")}</p>
      ) : (
        <>
          {inUse && <div className="callout warn">{t("load.exitFirst")}</div>}
          <div className="choice">
            <button className="choice-btn" disabled={busy || inUse} onClick={() => go("replace")}>
              <strong>{t("load.replace")}</strong>
              <span>{t("load.replaceHint")}</span>
            </button>
            <button className="choice-btn" disabled={busy} onClick={() => go("copy")}>
              <strong>{t("load.copy")}</strong>
              <span>{t("load.copyHint")}</span>
            </button>
          </div>
        </>
      )}
      <div className="actions">
        <button onClick={onClose}>{t("common.cancel")}</button>
        {missing && (
          <button className="primary" disabled={busy} onClick={() => go("replace")}>
            {busy ? t("load.restoring") : t("load.restore")}
          </button>
        )}
      </div>
    </Modal>
  );
}

function InfoPanel({ w }: { w: WorldInfo }) {
  const { t } = useI18n();
  const f = useFormat();
  const pos = w.player?.pos?.map((n) => Math.floor(n)).join(", ");
  const rules = Object.entries(w.gameRules).sort(([a], [b]) => a.localeCompare(b));
  return (
    <section className="panel">
      <h2>{t("info.world")}</h2>
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
