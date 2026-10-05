import { useState } from "react";
import { api, ApiError } from "../api";
import { AdvancementsPanel } from "../components/Advancements";
import { DeleteDialog, LoadDialog, useSnapTitle } from "../components/Snapshots";
import { KindBadge, WorldIcon } from "../components/ui";
import { WorldInfoPanel } from "../components/WorldInfoPanel";
import { useFormat } from "../format";
import { useLoad } from "../hooks";
import { useI18n } from "../i18n";
import { jobError, JobProgress, useWorldJob } from "../jobs";

// SnapshotDetail shows what a save contains, read from the backup itself,
// so it works even when the world is gone from this computer.
export function SnapshotDetail({ id, snapId, onBack }: { id: string; snapId: string; onBack: () => void }) {
  const { t, errorText } = useI18n();
  const f = useFormat();
  const title = useSnapTitle();
  const world = useLoad(() => api.world(id), [id], 5000);
  const snaps = useLoad(() => api.snapshots(id), [id]);
  const info = useLoad(() => api.snapshotInfo(id, snapId), [id, snapId]);
  const [dialog, setDialog] = useState<"load" | "delete">();
  const [notice, setNotice] = useState<{ ok: boolean; text: string }>();

  const missing = world.error instanceof ApiError && world.error.status === 404;
  const snap = snaps.data?.snapshots?.find((s) => s.id === snapId);
  const worldName = world.data?.name ?? snaps.data?.world.name ?? id;

  const { job, track } = useWorldJob(id, (j) => {
    world.reload();
    if (j.kind !== "restore" || j.auto) return;
    if (j.state === "failed") setNotice({ ok: false, text: errorText(jobError(j)) });
    else if (j.path) setNotice({ ok: true, text: t("detail.loadedInto", { path: j.path }) });
  });
  const run = async (fn: () => Promise<string | void>) => {
    try {
      const text = await fn();
      setDialog(undefined);
      if (text) setNotice({ ok: true, text });
      world.reload();
    } catch (e) {
      setNotice({ ok: false, text: errorText(e) });
      setDialog(undefined);
    }
  };

  if (snaps.loading) return <p className="muted">{t("common.loading")}</p>;
  if (!snap) {
    return (
      <>
        <button className="link" onClick={onBack}>
          {t("snapshot.backTo", { world: worldName })}
        </button>
        <div className="callout error">{snaps.error ? errorText(snaps.error) : t("snapshot.notFound")}</div>
      </>
    );
  }

  return (
    <>
      <button className="link" onClick={onBack}>
        {t("snapshot.backTo", { world: worldName })}
      </button>

      <header className="world-header">
        <WorldIcon id={id} hasIcon={world.data?.hasIcon} size={96} />
        <div>
          <div className="muted small">{t("snapshot.of", { world: worldName })}</div>
          <h1>{title(snap)}</h1>
          <div className="world-meta">
            <KindBadge kind={snap.kind} />
            {snap.gameVersion && <span className="badge">{snap.gameVersion}</span>}
            <span className="muted small">
              {t("snapshot.created")} {f.dateTime(snap.createdAt)} · {f.bytes(snap.sizeBytes)}
            </span>
          </div>
          {snap.note && <p className="snapshot-note">{snap.note}</p>}
        </div>
        <div className="header-actions">
          <button className="primary" disabled={!!job} onClick={() => setDialog("load")}>
            {t("common.load")}
          </button>
          <button disabled={!!job} onClick={() => setDialog("delete")}>
            {t("common.delete")}
          </button>
        </div>
      </header>

      {notice && (
        <div className={`callout ${notice.ok ? "success" : "error"}`} onClick={() => setNotice(undefined)}>
          {notice.text}
        </div>
      )}

      {job && <JobProgress job={job} />}

      {info.error ? (
        <div className="callout error">{errorText(info.error)}</div>
      ) : !info.data ? (
        <p className="muted">{t("snapshot.reading")}</p>
      ) : (
        <>
          <WorldInfoPanel w={info.data} title={t("snapshot.contents")} />
          <AdvancementsPanel
            load={() => api.snapshotAdvancements(id, snapId)}
            loadKey={`${id}/${snapId}`}
            earned={info.data.advancements}
          />
        </>
      )}

      {dialog === "load" && (
        <LoadDialog
          title={title(snap)}
          missing={missing}
          inUse={!!world.data?.inUse}
          onClose={() => setDialog(undefined)}
          onLoad={(mode) =>
            run(async () => {
              track(await api.restore(id, snapId, mode));
            })
          }
        />
      )}
      {dialog === "delete" && (
        <DeleteDialog
          title={title(snap)}
          onClose={() => setDialog(undefined)}
          onDelete={() =>
            run(async () => {
              await api.deleteSnapshot(id, snapId);
              onBack();
            })
          }
        />
      )}
    </>
  );
}
