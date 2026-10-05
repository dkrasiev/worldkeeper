import { api, type WorldView } from "../api";
import { ModeBadge, WorldIcon } from "../components/ui";
import { useFormat } from "../format";
import { useLoad } from "../hooks";
import { useI18n } from "../i18n";

function Status({ w }: { w: WorldView }) {
  const { t } = useI18n();
  const f = useFormat();
  if (w.error) return <span className="status error">{t("status.unreadable")}</span>;
  if (w.inUse) return <span className="status live">{t("status.inGame")}</span>;
  if (!w.lastBackup) return <span className="status warn">{t("status.noBackups")}</span>;
  if (w.changed) return <span className="status warn">{t("status.changed")}</span>;
  return <span className="status ok">{t("status.backedUp", { when: f.relative(w.lastBackup.createdAt) })}</span>;
}

export function WorldList({ onOpen }: { onOpen: (id: string) => void }) {
  const { t, errorText } = useI18n();
  const f = useFormat();
  const { data, error, loading } = useLoad(api.overview, [], 5000);

  if (loading && !data) return <p className="muted">{t("list.looking")}</p>;
  if (error && !data) return <div className="callout error">{errorText(error)}</div>;
  if (!data) return null;

  const groups = new Map<string, WorldView[]>();
  for (const w of [...data.worlds].sort((a, b) => b.lastPlayed.localeCompare(a.lastPlayed))) {
    groups.set(w.sourceLabel, [...(groups.get(w.sourceLabel) ?? []), w]);
  }

  return (
    <>
      {error && <div className="callout error">{t("list.connectionLost", { error: errorText(error) })}</div>}
      {data.storageError && (
        <div className="callout error">
          {t("list.storageError", { error: data.storageError })} <a href="#/settings">{t("list.checkSettings")}</a>
        </div>
      )}
      {data.worlds.length === 0 && <div className="callout">{t("list.noWorlds")}</div>}

      {[...groups].map(([label, worlds]) => (
        <section key={label}>
          <h2 className="section-title">{label}</h2>
          <div className="grid">
            {worlds.map((w) => (
              <button key={w.id} className="card world-card" onClick={() => onOpen(w.id)}>
                <WorldIcon id={w.id} hasIcon={w.hasIcon} />
                <div className="world-card-body">
                  <div className="world-name">{w.name}</div>
                  <div className="world-meta">
                    <ModeBadge mode={w.gameMode} hardcore={w.hardcore} />
                    {w.gameVersion && <span className="badge">{w.gameVersion}</span>}
                    {w.modded && <span className="badge">{t("badge.modded")}</span>}
                  </div>
                  <div className="world-sub">{t("list.played", { when: f.relative(w.lastPlayed) })}</div>
                  <Status w={w} />
                </div>
              </button>
            ))}
          </div>
        </section>
      ))}

      {data.archived.length > 0 && (
        <section>
          <h2 className="section-title">{t("list.archivedTitle")}</h2>
          <p className="muted small">{t("list.archivedHint")}</p>
          <div className="grid">
            {data.archived.map((ix) => (
              <button key={ix.world.id} className="card world-card archived" onClick={() => onOpen(ix.world.id)}>
                <WorldIcon />
                <div className="world-card-body">
                  <div className="world-name">{ix.world.name || ix.world.folder}</div>
                  <div className="world-sub">{ix.world.sourceLabel}</div>
                  <span className="status">
                    {t("list.savesCount", { count: ix.snapshots?.length ?? 0 })} ·{" "}
                    {t("list.lastSave", { when: f.relative(ix.snapshots?.[0]?.createdAt) })}
                  </span>
                </div>
              </button>
            ))}
          </div>
        </section>
      )}
    </>
  );
}
