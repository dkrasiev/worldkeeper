import { useState } from "react";
import { api, ApiError, type RestoreMode, type Snapshot, type WorldInfo } from "../api";
import { Field, KindBadge, Modal, ModeBadge, WorldIcon } from "../components/ui";
import { bytes, capitalize, dateTime, dimensionName, duration, num, relative } from "../format";
import { CopyText } from "../components/CopyText";
import { useLoad } from "../hooks";

type Dialog =
  | { type: "save"; force?: boolean }
  | { type: "load"; snap: Snapshot }
  | { type: "delete"; snap: Snapshot };

export function WorldDetail({ id, onBack }: { id: string; onBack: () => void }) {
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
      setNotice({ ok: false, text: (e as Error).message });
      setDialog(undefined);
    }
  };

  if (info.loading && snaps.loading) return <p className="muted">Loading…</p>;

  return (
    <>
      <button className="link" onClick={onBack}>
        ← All worlds
      </button>

      <header className="world-header">
        <WorldIcon id={id} hasIcon={world?.hasIcon} size={96} />
        <div>
          <h1>{world?.name ?? ref?.name ?? id}</h1>
          <div className="world-meta">
            {world && <ModeBadge mode={world.gameMode} hardcore={world.hardcore} />}
            {world?.gameVersion && <span className="badge">{world.gameVersion}</span>}
            {world?.inUse && <span className="status live">In game</span>}
            {missing && <span className="status warn">Not on this computer</span>}
          </div>
          <div className="muted small mono">{world?.path ?? ref?.path}</div>
        </div>
        {world && (
          <button className="primary" onClick={() => setDialog({ type: "save" })}>
            Save now
          </button>
        )}
      </header>

      {notice && (
        <div className={`callout ${notice.ok ? "success" : "error"}`} onClick={() => setNotice(undefined)}>
          {notice.text}
        </div>
      )}
      {info.error && !missing && <div className="callout error">{info.error.message}</div>}

      <div className="columns">
        <section className="panel">
          <h2>Saves</h2>
          {list.length === 0 && <p className="muted">No saves yet. They appear here after you close the world in the game.</p>}
          <ul className="snapshots">
            {list.map((s) => (
              <li key={s.id} className="snapshot">
                <div className="snapshot-main">
                  <div>
                    <KindBadge kind={s.kind} /> <strong>{s.label || dateTime(s.createdAt)}</strong>
                  </div>
                  {s.note && <div className="small">{s.note}</div>}
                  <div className="muted small">
                    {s.label && <>{dateTime(s.createdAt)} · </>}
                    {bytes(s.sizeBytes)}
                    {s.gameVersion && <> · {s.gameVersion}</>}
                  </div>
                </div>
                <div className="snapshot-actions">
                  <button onClick={() => setDialog({ type: "load", snap: s })}>Load</button>
                  <button className="ghost" title="Delete" onClick={() => setDialog({ type: "delete", snap: s })}>
                    ✕
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </section>

        {world && <InfoPanel w={world} />}
      </div>

      {dialog?.type === "save" && (
        <SaveDialog
          force={!!dialog.force}
          onClose={() => setDialog(undefined)}
          onSave={(label, note) =>
            run(async () => {
              const s = await api.save(id, label, note, dialog.force);
              return `Saved “${s.label || dateTime(s.createdAt)}”.`;
            })
          }
        />
      )}
      {dialog?.type === "load" && (
        <LoadDialog
          snap={dialog.snap}
          missing={missing}
          inUse={!!world?.inUse}
          onClose={() => setDialog(undefined)}
          onLoad={(mode) =>
            run(async () => {
              const r = await api.restore(id, dialog.snap.id, mode);
              return `Loaded into ${r.path}`;
            })
          }
        />
      )}
      {dialog?.type === "delete" && (
        <Modal title="Delete this save?" onClose={() => setDialog(undefined)}>
          <p>
            {dialog.snap.label || dateTime(dialog.snap.createdAt)} will be removed from storage. This cannot be undone.
          </p>
          <div className="actions">
            <button onClick={() => setDialog(undefined)}>Cancel</button>
            <button
              className="danger"
              onClick={() =>
                run(async () => {
                  await api.deleteSnapshot(id, dialog.snap.id);
                  return "Save deleted.";
                })
              }
            >
              Delete
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
  const [label, setLabel] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <Modal title="Save world" onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setBusy(true);
          onSave(label, note);
        }}
      >
        {force && (
          <div className="callout warn">
            The world is open in the game right now. The game keeps writing to it, so this save may be inconsistent.
            For a clean save, exit to the title screen first.
          </div>
        )}
        <label>
          Name
          <input autoFocus value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Before the Ender Dragon" />
        </label>
        <label>
          Note
          <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} placeholder="Optional" />
        </label>
        <div className="actions">
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button className={force ? "danger" : "primary"} disabled={busy}>
            {busy ? "Saving…" : force ? "Save anyway" : "Save"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function LoadDialog({
  snap,
  missing,
  inUse,
  onClose,
  onLoad,
}: {
  snap: Snapshot;
  missing: boolean;
  inUse: boolean;
  onClose: () => void;
  onLoad: (mode: RestoreMode) => void;
}) {
  const [busy, setBusy] = useState(false);
  const go = (mode: RestoreMode) => {
    setBusy(true);
    onLoad(mode);
  };
  return (
    <Modal title={`Load “${snap.label || dateTime(snap.createdAt)}”`} onClose={onClose}>
      {missing ? (
        <p>The world will be restored to its original folder, or into the default saves folder if that is taken.</p>
      ) : (
        <>
          {inUse && <div className="callout warn">Exit the world in the game before replacing it.</div>}
          <div className="choice">
            <button className="choice-btn" disabled={busy || inUse} onClick={() => go("replace")}>
              <strong>Replace current world</strong>
              <span>The current state is saved first as a “Safety” save, so you can always go back.</span>
            </button>
            <button className="choice-btn" disabled={busy} onClick={() => go("copy")}>
              <strong>Load as a new world</strong>
              <span>Unpacks next to the original. Nothing is overwritten.</span>
            </button>
          </div>
        </>
      )}
      <div className="actions">
        <button onClick={onClose}>Cancel</button>
        {missing && (
          <button className="primary" disabled={busy} onClick={() => go("replace")}>
            {busy ? "Restoring…" : "Restore"}
          </button>
        )}
      </div>
    </Modal>
  );
}

function InfoPanel({ w }: { w: WorldInfo }) {
  const pos = w.player?.pos?.map((n) => Math.floor(n)).join(", ");
  const rules = Object.entries(w.gameRules).sort(([a], [b]) => a.localeCompare(b));
  return (
    <section className="panel">
      <h2>World</h2>
      <dl className="fields">
        <Field label="Last played">
          {dateTime(w.lastPlayed)} <span className="muted">({relative(w.lastPlayed)})</span>
        </Field>
        <Field label="Difficulty">
          {capitalize(w.difficulty)}
          {w.difficultyLocked && " (locked)"}
        </Field>
        <Field label="Cheats">{w.cheats ? "On" : "Off"}</Field>
        <Field label="Seed">
          {w.seed ? <CopyText text={w.seed} /> : "—"}
        </Field>
        <Field label="Day">{num(w.day)}</Field>
        <Field label="Weather">{capitalize(w.weather)}</Field>
        {w.spawn && <Field label="World spawn">{w.spawn.join(", ")}</Field>}
        <Field label="Size on disk">{bytes(w.sizeBytes)}</Field>
        {w.brands.length > 0 && <Field label="Loaders">{w.brands.join(", ")}</Field>}
      </dl>

      {w.player && (
        <>
          <h3>Player</h3>
          <dl className="fields">
            <Field label="Dimension">{dimensionName(w.player.dimension)}</Field>
            {pos && <Field label="Position">{pos}</Field>}
            <Field label="Health">{w.player.health / 2} ❤</Field>
            <Field label="Food">{w.player.food / 2} 🍗</Field>
            <Field label="Level">{w.player.xpLevel}</Field>
          </dl>
        </>
      )}

      {w.stats && (
        <>
          <h3>Statistics</h3>
          <dl className="fields">
            <Field label="Play time">{duration(w.stats.playTimeSeconds)}</Field>
            <Field label="Deaths">{num(w.stats.deaths)}</Field>
            <Field label="Mobs killed">{num(w.stats.mobKills)}</Field>
            <Field label="Distance travelled">{num(Math.round(w.stats.distanceMeters / 1000))} km</Field>
            <Field label="Jumps">{num(w.stats.jumps)}</Field>
            <Field label="Advancements">{w.advancements}</Field>
          </dl>
        </>
      )}

      <h3>Dimensions</h3>
      <dl className="fields">
        {w.dimensions.map((d) => (
          <Field key={d.id} label={dimensionName(d.id)}>
            {d.regionFiles} region{d.regionFiles === 1 ? "" : "s"}{" "}
            <span className="muted">(~{num(d.regionFiles * 0.262144)} km²)</span>
          </Field>
        ))}
      </dl>

      {(w.dataPacks.enabled.length > 0 || rules.length > 0) && (
        <>
          <details>
            <summary>Data packs ({w.dataPacks.enabled.length})</summary>
            <ul className="plain">
              {w.dataPacks.enabled.map((p) => (
                <li key={p} className="mono small">
                  {p}
                </li>
              ))}
            </ul>
          </details>
          <details>
            <summary>Game rules ({rules.length})</summary>
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
