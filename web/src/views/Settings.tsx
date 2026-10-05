import { useEffect, useState } from "react";
import { api, type Settings } from "../api";
import { useLoad } from "../hooks";

export function SettingsView() {
  const { data, error } = useLoad(api.settings, []);
  const [form, setForm] = useState<Settings>();
  const [extra, setExtra] = useState("");
  const [status, setStatus] = useState<{ ok: boolean; text: string }>();

  useEffect(() => {
    if (data) {
      setForm(data);
      setExtra(data.extraSavesDirs.join("\n"));
    }
  }, [data]);

  if (error) return <div className="callout error">{error.message}</div>;
  if (!form) return <p className="muted">Loading…</p>;

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setForm({ ...form, [k]: v });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const saved = await api.saveSettings({ ...form, extraSavesDirs: extra.split("\n").map((s) => s.trim()).filter(Boolean) });
      setForm(saved);
      setStatus({ ok: true, text: "Settings saved." });
    } catch (err) {
      setStatus({ ok: false, text: (err as Error).message });
    }
  };

  return (
    <form className="panel settings" onSubmit={submit}>
      <h2>Settings</h2>

      <label>
        Backup folder
        <input className="mono" value={form.storageDir} onChange={(e) => set("storageDir", e.target.value)} />
        <span className="hint">
          A local folder, a network share (<code>\\NAS\backups\minecraft</code>, <code>/Volumes/backups</code>) or a
          Google Drive / OneDrive / Dropbox folder.
        </span>
      </label>

      <label className="check">
        <input type="checkbox" checked={form.autoBackup} onChange={(e) => set("autoBackup", e.target.checked)} />
        Back up a world automatically when you exit it in the game
      </label>

      <div className="row">
        <label>
          Automatic saves to keep per world
          <input type="number" min={1} value={form.keepAuto} onChange={(e) => set("keepAuto", Number(e.target.value))} />
          <span className="hint">Your named saves are never deleted automatically.</span>
        </label>
        <label>
          Check every (seconds)
          <input type="number" min={5} value={form.pollSeconds} onChange={(e) => set("pollSeconds", Number(e.target.value))} />
        </label>
      </div>

      <label>
        <span>
          Extra <code>saves</code> folders
        </span>
        <textarea className="mono" rows={3} value={extra} onChange={(e) => setExtra(e.target.value)} placeholder="One path per line" />
        <span className="hint">For launchers Worldkeeper does not know about.</span>
      </label>

      {status && <div className={`callout ${status.ok ? "success" : "error"}`}>{status.text}</div>}
      <div className="actions">
        <button className="primary">Save settings</button>
      </div>
    </form>
  );
}
