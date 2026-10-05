import { useEffect, useState } from "react";
import { api } from "../api";
import { useI18n } from "../i18n";

// The advancement tree is drawn by mcwidgets (github.com/KabanFriends/mcwidgets),
// loaded from its public host in a cross-origin iframe: the game textures it
// needs are Mojang's, so Worldkeeper does not ship them. The iframe cannot
// reach this page or its API token.
const HOST = "mcwidgets.kaban.sh";
const EMBED = `https://${HOST}/advancements/embed.html`;
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
  const { t, errorText } = useI18n();
  const [enabled, setEnabled] = useState(readOptIn);
  const [src, setSrc] = useState<string>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setError(undefined);
    api.advancements(id).then(
      (p) => !cancelled && setSrc(embedUrl(p)),
      (e: unknown) => !cancelled && setError(errorText(e)),
    );
    return () => {
      cancelled = true;
    };
  }, [enabled, id, errorText]);

  const toggle = (on: boolean) => {
    writeOptIn(on);
    setEnabled(on);
    if (!on) setSrc(undefined);
  };

  return (
    <section className="panel advancements">
      <div className="panel-head">
        <h2>{t("adv.title")}</h2>
        <span className="muted">{t("adv.earned", { count: earned })}</span>
        {enabled && (
          <button type="button" className="link small" onClick={() => toggle(false)}>
            {t("adv.hide")}
          </button>
        )}
      </div>

      {!enabled ? (
        <div className="adv-optin">
          <p>
            {t("adv.optinBefore")}{" "}
            <a href="https://github.com/KabanFriends/mcwidgets" target="_blank" rel="noreferrer">
              mcwidgets
            </a>{" "}
            {t("adv.optinAfter", { host: HOST })}
          </p>
          <button type="button" className="primary" onClick={() => toggle(true)}>
            {t("adv.show")}
          </button>
        </div>
      ) : error ? (
        <div className="callout error">{error}</div>
      ) : !src ? (
        <p className="muted">{t("common.loading")}</p>
      ) : (
        <>
          <iframe
            className="adv-frame"
            src={src}
            title={t("adv.frameTitle")}
            sandbox="allow-scripts allow-same-origin"
            referrerPolicy="no-referrer"
            loading="lazy"
          />
          <p className="muted small">{t("adv.note")}</p>
        </>
      )}
    </section>
  );
}
