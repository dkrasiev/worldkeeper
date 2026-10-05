import { useEffect, useState } from "react";
import { api, type Engine, type ResticStatus, type Settings } from "../api";
import { useLoad } from "../hooks";

const resticStateText: Record<ResticStatus["state"], string> = {
  ok: "Repository is ready",
  not_installed: "restic is not installed",
  missing: "Repository does not exist yet",
  wrong_password: "Wrong password",
  no_password: "Password is not set",
  error: "Cannot open the repository",
};

export function SettingsView() {
  const { data, error } = useLoad(api.settings, []);
  const [form, setForm] = useState<Settings>();
  const [extra, setExtra] = useState("");
  const [password, setPassword] = useState("");
  const [status, setStatus] = useState<{ ok: boolean; text: string }>();
  const [restic, setRestic] = useState<ResticStatus>();
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (data) {
      setForm(data);
      setExtra((data.extraSavesDirs ?? []).join("\n"));
      if (data.engine === "restic") api.resticCheck().then(setRestic, () => {});
    }
  }, [data]);

  if (error) return <div className="callout error">{error.message}</div>;
  if (!form) return <p className="muted">Loading…</p>;

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setForm({ ...form, [k]: v });
  const setResticField = (k: keyof Settings["restic"], v: string) => setForm({ ...form, restic: { ...form.restic, [k]: v } });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const saved = await api.saveSettings({
        ...form,
        resticPassword: password || undefined,
        extraSavesDirs: extra
          .split("\n")
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setForm(saved);
      setPassword("");
      setStatus({ ok: true, text: "Settings saved." });
      setRestic(saved.engine === "restic" ? await api.resticCheck() : undefined);
    } catch (err) {
      setStatus({ ok: false, text: (err as Error).message });
    } finally {
      setBusy(false);
    }
  };

  const init = async () => {
    setBusy(true);
    try {
      setRestic(await api.resticInit());
    } catch (err) {
      setRestic({ state: "error", message: (err as Error).message });
    } finally {
      setBusy(false);
    }
  };

  const engineOption = (value: Engine, title: string, text: string) => (
    <button
      type="button"
      className={`choice-btn ${form.engine === value ? "selected" : ""}`}
      aria-pressed={form.engine === value}
      onClick={() => set("engine", value)}
    >
      <strong>{title}</strong>
      <span>{text}</span>
    </button>
  );

  return (
    <form className="panel settings" onSubmit={submit}>
      <h2>Settings</h2>

      <div className="field-group">
        <div className="group-label">Storage format</div>
        <div className="choice two">
          {engineOption("zip", "Zip files", "One zip per save. No password, open with any archive tool.")}
          {engineOption(
            "restic",
            "Restic repository",
            "Encrypted, deduplicated: many saves of a big world cost little space. Needs restic installed.",
          )}
        </div>
      </div>

      {form.engine === "zip" ? (
        <label>
          Backup folder
          <input className="mono" value={form.storageDir} onChange={(e) => set("storageDir", e.target.value)} />
          <span className="hint">
            A local folder, a network share (<code>\\NAS\backups\minecraft</code>, <code>/Volumes/backups</code>) or a
            Google Drive / OneDrive / Dropbox folder.
          </span>
        </label>
      ) : (
        <>
          <label>
            Repository
            <input
              className="mono"
              value={form.restic.repo}
              onChange={(e) => setResticField("repo", e.target.value)}
              placeholder="\\NAS\backups\restic  or  sftp:user@nas:/backups/restic"
            />
            <span className="hint">
              Anything restic accepts: a folder or network share, <code>sftp:</code>, <code>rest:</code>, …
            </span>
          </label>

          <label>
            Password
            <input
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={form.resticPasswordSet ? "Saved — type to replace" : "Repository password"}
            />
            <span className="hint">Stored in your system keychain, not in the config file.</span>
          </label>
          <div className="callout warn">
            Write the password down somewhere safe. Without it the backups can never be restored — not by Worldkeeper,
            not by restic, not by anyone.
          </div>

          <details>
            <summary>Advanced</summary>
            <label>
              restic executable
              <input
                className="mono"
                value={form.restic.binary}
                onChange={(e) => setResticField("binary", e.target.value)}
                placeholder="restic (from PATH)"
              />
            </label>
            <label>
              Password file
              <input
                className="mono"
                value={form.restic.passwordFile}
                onChange={(e) => setResticField("passwordFile", e.target.value)}
                placeholder="Use instead of the keychain"
              />
            </label>
          </details>

          {restic && (
            <div className={`callout ${restic.state === "ok" ? "success" : restic.state === "missing" ? "warn" : "error"}`}>
              <strong>{resticStateText[restic.state]}</strong>
              {restic.version && <span className="muted"> · restic {restic.version}</span>}
              {restic.message && restic.state !== "ok" && <div className="small">{restic.message}</div>}
              {restic.state === "not_installed" && (
                <div className="small">
                  Install it with <code>winget install restic.restic</code> (Windows) or <code>brew install restic</code>{" "}
                  (macOS), then restart Worldkeeper.
                </div>
              )}
              {restic.state === "missing" && (
                <div className="actions">
                  <button type="button" className="primary" disabled={busy} onClick={init}>
                    Initialize repository
                  </button>
                </div>
              )}
            </div>
          )}
        </>
      )}

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
        <button className="primary" disabled={busy}>
          {busy ? "Saving…" : "Save settings"}
        </button>
      </div>
    </form>
  );
}
