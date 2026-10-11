import { useState } from "react";
import { token } from "./api";
import { LOCALES, useI18n, type Locale } from "./i18n";
import { useHashRoute, useLoad } from "./hooks";
import { JobsIndicator } from "./jobs";
import { api } from "./api";
import { Activity } from "./views/Activity";
import { SettingsView } from "./views/Settings";
import { SnapshotDetail } from "./views/SnapshotDetail";
import { WorldDetail } from "./views/WorldDetail";
import { WorldList } from "./views/WorldList";

export function App() {
  const { t, locale, setLocale } = useI18n();
  const [route, go] = useHashRoute();
  const [page, arg, sub, subArg] = route;

  const tab = (name: string, path: string, label: string) => (
    <a className={`tab ${(page ?? "") === name ? "active" : ""}`} href={`#/${path}`}>
      {label}
    </a>
  );

  return (
    <div className="shell">
      <header className="topbar">
        <a className="brand" href="#/">
          <span className="brand-block" aria-hidden />
          Worldkeeper
        </a>
        {token && <JobsIndicator />}
        <nav className="tabs">
          {tab("", "", t("nav.worlds"))}
          {tab("activity", "activity", t("nav.activity"))}
          {tab("settings", "settings", t("nav.settings"))}
          <select
            className="lang"
            aria-label={t("lang.label")}
            value={locale}
            onChange={(e) => setLocale(e.target.value as Locale)}
          >
            {LOCALES.map((l) => (
              <option key={l.code} value={l.code}>
                {l.name}
              </option>
            ))}
          </select>
        </nav>
      </header>

      <main className="content">
        {token && <UpdateBanner />}
        {!token ? (
          <div className="callout error">{t("app.noToken")}</div>
        ) : page === "world" && arg && sub === "save" && subArg ? (
          <SnapshotDetail
            key={`${arg}/${subArg}`}
            id={arg}
            snapId={subArg}
            onBack={() => go(`/world/${encodeURIComponent(arg)}`)}
          />
        ) : page === "world" && arg ? (
          <WorldDetail key={arg} id={arg} onBack={() => go("/")} />
        ) : page === "activity" ? (
          <Activity />
        ) : page === "settings" ? (
          <SettingsView />
        ) : (
          <WorldList onOpen={(id) => go(`/world/${encodeURIComponent(id)}`)} />
        )}
      </main>

      {token && <Footer />}
    </div>
  );
}

const DISMISSED_KEY = "worldkeeper.updateDismissed";

// UpdateBanner announces a newer release until it is installed or the user
// dismisses that version; the next release shows it again.
function UpdateBanner() {
  const { t } = useI18n();
  const { data } = useLoad(api.update, [], 10 * 60_000);
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(DISMISSED_KEY) ?? "";
    } catch {
      return "";
    }
  });
  if (!data?.available || !data.latest || dismissed === data.latest.version) return null;
  const { version, dev } = data.latest;
  const dismiss = () => {
    try {
      localStorage.setItem(DISMISSED_KEY, version);
    } catch {
      /* storage blocked: hide until reload */
    }
    setDismissed(version);
  };
  return (
    <div className="callout update">
      <span>{t(dev ? "update.availableDev" : "update.available", { version, current: data.current })}</span>
      <a href={data.latest.download ?? data.latest.url} target="_blank" rel="noreferrer">
        {t("update.download")}
      </a>
      <button type="button" className="link" onClick={dismiss}>
        {t("update.later")}
      </button>
    </div>
  );
}

const REPO = "https://github.com/dkrasiev/worldkeeper";

// Footer shows the running version on every page. Release versions link to
// their release notes; test builds (x.y.z-<branch>.<time>.<commit>) and
// local builds ("dev") are labelled so they are not mistaken for a release.
function Footer() {
  const { t } = useI18n();
  const { data } = useLoad(api.about, []);
  if (!data) return null;
  const v = data.version;
  const release = /^\d+\.\d+\.\d+$/.test(v);
  const testBuild = /^\d+\.\d+\.\d+-/.test(v);
  return (
    <footer className="footer">
      <span>
        Worldkeeper <span className="mono">{v}</span>
        {v === "dev" && <span className="badge">{t("about.devBuild")}</span>}
        {testBuild && <span className="badge">{t("about.testBuild")}</span>}
      </span>
      {release && (
        <a href={`${REPO}/releases/tag/v${v}`} target="_blank" rel="noreferrer">
          {t("about.releaseNotes")}
        </a>
      )}
      <a href={REPO} target="_blank" rel="noreferrer">
        GitHub
      </a>
      <a href={`${REPO}/issues/new/choose`} target="_blank" rel="noreferrer">
        {t("about.report")}
      </a>
    </footer>
  );
}
