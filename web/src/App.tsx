import { token } from "./api";
import { useHashRoute } from "./hooks";
import { Activity } from "./views/Activity";
import { SettingsView } from "./views/Settings";
import { WorldDetail } from "./views/WorldDetail";
import { WorldList } from "./views/WorldList";

export function App() {
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
          {tab("", "", "Worlds")}
          {tab("activity", "activity", "Activity")}
          {tab("settings", "settings", "Settings")}
        </nav>
      </header>

      <main className="content">
        {!token ? (
          <div className="callout error">
            No access token. Open Worldkeeper from the link it prints on start (or run it again — it opens the
            browser for you).
          </div>
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
