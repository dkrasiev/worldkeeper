import { api, type WorldView } from "../api";
import { ModeBadge, WorldIcon } from "../components/ui";
import { relative } from "../format";
import { useLoad } from "../hooks";

function Status({ w }: { w: WorldView }) {
  if (w.error) return <span className="status error">Unreadable</span>;
  if (w.inUse) return <span className="status live">In game</span>;
  if (!w.lastBackup) return <span className="status warn">No backups</span>;
  if (w.changed) return <span className="status warn">Changed since backup</span>;
  return <span className="status ok">Backed up {relative(w.lastBackup.createdAt)}</span>;
}

export function WorldList({ onOpen }: { onOpen: (id: string) => void }) {
  const { data, error, loading } = useLoad(api.overview, [], 5000);

  if (loading && !data) return <p className="muted">Looking for worlds…</p>;
  if (error && !data) return <div className="callout error">{error.message}</div>;
  if (!data) return null;

  const groups = new Map<string, WorldView[]>();
  for (const w of [...data.worlds].sort((a, b) => b.lastPlayed.localeCompare(a.lastPlayed))) {
    groups.set(w.sourceLabel, [...(groups.get(w.sourceLabel) ?? []), w]);
  }

  return (
    <>
      {error && <div className="callout error">Connection lost: {error.message}</div>}
      {data.worlds.length === 0 && (
        <div className="callout">
          No worlds found in the usual launcher folders. Add your <code>saves</code> folder in Settings.
        </div>
      )}

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
                    {w.modded && <span className="badge">Modded</span>}
                  </div>
                  <div className="world-sub">Played {relative(w.lastPlayed)}</div>
                  <Status w={w} />
                </div>
              </button>
            ))}
          </div>
        </section>
      ))}

      {data.archived.length > 0 && (
        <section>
          <h2 className="section-title">Backups without a local world</h2>
          <p className="muted small">
            These worlds are not on this computer anymore (for example after reinstalling the OS). Open one to restore
            it.
          </p>
          <div className="grid">
            {data.archived.map((ix) => (
              <button key={ix.world.id} className="card world-card archived" onClick={() => onOpen(ix.world.id)}>
                <WorldIcon />
                <div className="world-card-body">
                  <div className="world-name">{ix.world.name || ix.world.folder}</div>
                  <div className="world-sub">{ix.world.sourceLabel}</div>
                  <span className="status">
                    {ix.snapshots?.length ?? 0} saves · last {relative(ix.snapshots?.[0]?.createdAt)}
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
