import { useState } from "react";
import { api, ApiError, type Snapshot } from "../api";
import { AdvancementsPanel } from "../components/Advancements";
import { DeleteDialog, LoadDialog, useSnapTitle } from "../components/Snapshots";
import { KindBadge, Modal, ModeBadge, WorldIcon } from "../components/ui";
import { WorldInfoPanel } from "../components/WorldInfoPanel";
import { useFormat } from "../format";
import { useLoad } from "../hooks";
import { useI18n } from "../i18n";
import { jobError, JobProgress, useWorldJob } from "../jobs";

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
  const { job, track } = useWorldJob(id, (j) => {
    refresh();
    if (j.auto) return; // automatic backups report in the Activity tab
    if (j.state === "failed") setNotice({ ok: false, text: errorText(jobError(j)) });
    else if (j.kind === "backup" && j.snapshot) setNotice({ ok: true, text: t("detail.saved", { name: title(j.snapshot) }) });
    else if (j.path) setNotice({ ok: true, text: t("detail.loadedInto", { path: j.path }) });
  });
  const run = async (fn: () => Promise<string | void>) => {
    try {
      const text = await fn();
      setNotice(text ? { ok: true, text } : undefined);
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
          <button className="primary" disabled={!!job} onClick={() => setDialog({ type: "save" })}>
            {t("detail.saveNow")}
          </button>
        )}
      </header>

      {notice && (
        <div className={`callout ${notice.ok ? "success" : "error"}`} onClick={() => setNotice(undefined)}>
          {notice.text}
        </div>
      )}
      {job && <JobProgress job={job} />}
      {info.error && !missing && <div className="callout error">{errorText(info.error)}</div>}

      <div className="columns">
        <section className="panel">
          <h2>{t("detail.saves")}</h2>
          {list.length === 0 && <p className="muted">{t("detail.noSaves")}</p>}
          <ul className="snapshots">
            {list.map((s) => (
              <li key={s.id} className="snapshot">
                <a
                  className="snapshot-main snapshot-link"
                  href={`#/world/${encodeURIComponent(id)}/save/${encodeURIComponent(s.id)}`}
                  title={t("snapshot.open")}
                >
                  <div>
                    <KindBadge kind={s.kind} /> <strong>{title(s)}</strong>
                  </div>
                  {s.note && <div className="small">{s.note}</div>}
                  <div className="muted small">
                    {s.label && <>{f.dateTime(s.createdAt)} · </>}
                    {f.bytes(s.sizeBytes)}
                    {s.gameVersion && <> · {s.gameVersion}</>}
                  </div>
                </a>
                <div className="snapshot-actions">
                  <button disabled={!!job} onClick={() => setDialog({ type: "load", snap: s })}>
                    {t("common.load")}
                  </button>
                  <button className="ghost" disabled={!!job} title={t("common.delete")} aria-label={t("common.delete")} onClick={() => setDialog({ type: "delete", snap: s })}>
                    ✕
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </section>

        {world && <WorldInfoPanel w={world} />}
      </div>

      {world && <AdvancementsPanel load={() => api.advancements(id)} loadKey={id} earned={world.advancements} />}

      {dialog?.type === "save" && (
        <SaveDialog
          force={!!dialog.force}
          onClose={() => setDialog(undefined)}
          onSave={(label, note) =>
            run(async () => {
              track(await api.save(id, label, note, dialog.force));
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
              track(await api.restore(id, dialog.snap.id, mode));
            })
          }
        />
      )}
      {dialog?.type === "delete" && (
        <DeleteDialog
          title={title(dialog.snap)}
          onClose={() => setDialog(undefined)}
          onDelete={() =>
            run(async () => {
              await api.deleteSnapshot(id, dialog.snap.id);
              return t("detail.deleted");
            })
          }
        />
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

