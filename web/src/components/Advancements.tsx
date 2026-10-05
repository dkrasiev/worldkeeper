import { useEffect, useState } from "react";
import { api } from "../api";

// The advancement tree is drawn by mcwidgets (github.com/KabanFriends/mcwidgets),
// loaded from its public host in a cross-origin iframe: the game textures it
// needs are Mojang's, so Worldkeeper does not ship them. The iframe cannot
// reach this page or its API token.
const EMBED = "https://mcwidgets.kaban.sh/advancements/embed.html";
const BUNDLE = "/bundles/vanilla/bundle.json";
const OPT_IN_KEY = "worldkeeper.advancementTree";

function readOptIn(): boolean {
  try {
    return localStorage.getItem(OPT_IN_KEY) === "1";
  } catch {
    return false;
  }
}

function writeOptIn(on: boolean) {
  try {
    if (on) localStorage.setItem(OPT_IN_KEY, "1");
    else localStorage.removeItem(OPT_IN_KEY);
  } catch {
    /* storage blocked: ask again next time */
  }
}

// The widget fetches progressUrl with fetch(), so a data: URL works and the
// iframe never has to call back into the local API.
function embedUrl(progress: Record<string, unknown>): string {
  const q = new URLSearchParams({
    bundle: BUNDLE,
    progressUrl: "data:application/json," + encodeURIComponent(JSON.stringify(progress)),
  });
  return `${EMBED}?${q}`;
}

export function AdvancementsPanel({ id, earned }: { id: string; earned: number }) {
  const [enabled, setEnabled] = useState(readOptIn);
  const [src, setSrc] = useState<string>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setError(undefined);
    api.advancements(id).then(
      (p) => !cancelled && setSrc(embedUrl(p)),
      (e: Error) => !cancelled && setError(e.message),
    );
    return () => {
      cancelled = true;
    };
  }, [enabled, id]);

  const toggle = (on: boolean) => {
    writeOptIn(on);
    setEnabled(on);
    if (!on) setSrc(undefined);
  };

  return (
    <section className="panel advancements">
      <div className="panel-head">
        <h2>Advancements</h2>
        <span className="muted">{earned} earned</span>
        {enabled && (
          <button type="button" className="link small" onClick={() => toggle(false)}>
            Hide tree
          </button>
        )}
      </div>

      {!enabled ? (
        <div className="adv-optin">
          <p>
            Show the in-game advancement screen with this world's progress. It is drawn by{" "}
            <a href="https://github.com/KabanFriends/mcwidgets" target="_blank" rel="noreferrer">
              mcwidgets
            </a>{" "}
            from <code>mcwidgets.kaban.sh</code>, so it needs an internet connection, and your advancement progress is
            sent to that site to draw it.
          </p>
          <button type="button" className="primary" onClick={() => toggle(true)}>
            Show advancement tree
          </button>
        </div>
      ) : error ? (
        <div className="callout error">{error}</div>
      ) : !src ? (
        <p className="muted">Loading…</p>
      ) : (
        <>
          <iframe
            className="adv-frame"
            src={src}
            title="Minecraft advancements"
            sandbox="allow-scripts allow-same-origin"
            referrerPolicy="no-referrer"
            loading="lazy"
          />
          <p className="muted small">
            Vanilla advancements only; mod and datapack advancements are not shown. Hidden advancements appear once
            earned, like in the game. If the tree stays empty, check your internet connection.
          </p>
        </>
      )}
    </section>
  );
}
