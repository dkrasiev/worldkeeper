import { token } from "./api";
import { LOCALES, useI18n, type Locale } from "./i18n";
import { useHashRoute } from "./hooks";
import { Activity } from "./views/Activity";
import { SettingsView } from "./views/Settings";
import { WorldDetail } from "./views/WorldDetail";
import { WorldList } from "./views/WorldList";

export function App() {
  const { t, locale, setLocale } = useI18n();
  const [route, go] = useHashRoute();
  const [page, arg] = route;

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
        {!token ? (
          <div className="callout error">{t("app.noToken")}</div>
        ) : page === "world" && arg ? (
          <WorldDetail id={arg} onBack={() => go("/")} />
        ) : page === "activity" ? (
          <Activity />
        ) : page === "settings" ? (
          <SettingsView />
        ) : (
          <WorldList onOpen={(id) => go(`/world/${encodeURIComponent(id)}`)} />
        )}
      </main>
    </div>
  );
}
