import { useEffect, useState } from "react";
import { api, type Engine, type ResticStatus, type Settings } from "../api";
import { useLoad } from "../hooks";
import { useI18n } from "../i18n";

export function SettingsView() {
  const { t, errorText } = useI18n();
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

  if (error) return <div className="callout error">{errorText(error)}</div>;
  if (!form) return <p className="muted">{t("common.loading")}</p>;

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
      setStatus({ ok: true, text: t("settings.saved") });
      setRestic(saved.engine === "restic" ? await api.resticCheck() : undefined);
    } catch (err) {
      setStatus({ ok: false, text: errorText(err) });
    } finally {
      setBusy(false);
    }
  };

  const init = async () => {
    setBusy(true);
    try {
      setRestic(await api.resticInit());
    } catch (err) {
      setRestic({ state: "error", message: errorText(err) });
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
      <h2>{t("settings.title")}</h2>

      <div className="field-group">
        <div className="group-label">{t("settings.format")}</div>
        <div className="choice two">
          {engineOption("zip", t("settings.zipTitle"), t("settings.zipText"))}
          {engineOption("restic", t("settings.resticTitle"), t("settings.resticText"))}
        </div>
      </div>

      {form.engine === "zip" ? (
        <label>
          {t("settings.folder")}
          <input className="mono" value={form.storageDir} onChange={(e) => set("storageDir", e.target.value)} />
          <span className="hint">{t("settings.folderHint")}</span>
        </label>
      ) : (
        <>
          <label>
            {t("settings.repo")}
            <input
              className="mono"
              value={form.restic.repo}
              onChange={(e) => setResticField("repo", e.target.value)}
              placeholder="\\NAS\backups\restic  or  sftp:user@nas:/backups/restic"
            />
            <span className="hint">{t("settings.repoHint")}</span>
          </label>

          <label>
            {t("settings.password")}
            <input
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={form.resticPasswordSet ? t("settings.passwordSaved") : t("settings.passwordPlaceholder")}
            />
            <span className="hint">{t("settings.passwordHint")}</span>
          </label>
          <div className="callout warn">{t("settings.passwordWarning")}</div>

          <details>
            <summary>{t("settings.advanced")}</summary>
            <label>
              {t("settings.binary")}
              <input
                className="mono"
                value={form.restic.binary}
                onChange={(e) => setResticField("binary", e.target.value)}
                placeholder={t("settings.binaryPlaceholder")}
              />
            </label>
            <label>
              {t("settings.passwordFile")}
              <input
                className="mono"
                value={form.restic.passwordFile}
                onChange={(e) => setResticField("passwordFile", e.target.value)}
                placeholder={t("settings.passwordFilePlaceholder")}
              />
            </label>
          </details>

          {restic && (
            <div className={`callout ${restic.state === "ok" ? "success" : restic.state === "missing" ? "warn" : "error"}`}>
              <strong>{t(`restic.state.${restic.state}`)}</strong>
              {restic.version && <span className="muted"> · {t("restic.version", { version: restic.version })}</span>}
              {restic.state === "error" && restic.message && <div className="small">{restic.message}</div>}
              {restic.state === "missing" && <div className="small">{t("restic.missingHint")}</div>}
              {restic.state === "not_installed" && <div className="small">{t("restic.install")}</div>}
              {restic.state === "missing" && (
                <div className="actions">
                  <button type="button" className="primary" disabled={busy} onClick={init}>
                    {t("settings.init")}
                  </button>
                </div>
              )}
            </div>
          )}
        </>
      )}

      <label className="check">
        <input type="checkbox" checked={form.autoBackup} onChange={(e) => set("autoBackup", e.target.checked)} />
        {t("settings.autoBackup")}
      </label>

      <div className="row">
        <label>
          {t("settings.keepAuto")}
          <input type="number" min={1} value={form.keepAuto} onChange={(e) => set("keepAuto", Number(e.target.value))} />
          <span className="hint">{t("settings.keepAutoHint")}</span>
        </label>
        <label>
          {t("settings.poll")}
          <input type="number" min={5} value={form.pollSeconds} onChange={(e) => set("pollSeconds", Number(e.target.value))} />
        </label>
      </div>

      <label>
        {t("settings.extra")}
        <textarea className="mono" rows={3} value={extra} onChange={(e) => setExtra(e.target.value)} placeholder={t("settings.extraPlaceholder")} />
        <span className="hint">{t("settings.extraHint")}</span>
      </label>

      {status && <div className={`callout ${status.ok ? "success" : "error"}`}>{status.text}</div>}
      <div className="actions">
        <button className="primary" disabled={busy}>
          {busy ? t("settings.saving") : t("settings.save")}
        </button>
      </div>
    </form>
  );
}
