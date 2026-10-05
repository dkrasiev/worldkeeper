import { api, type ActivityEvent } from "../api";
import { useFormat } from "../format";
import { useLoad } from "../hooks";
import { useI18n, type I18n } from "../i18n";

// Events carry a code and params so they render in the UI language; the
// English message is the fallback for codes this UI does not know.
function eventText(e: ActivityEvent, tryT: I18n["tryT"]): string {
  const p = e.params ?? {};
  const text =
    e.code === "backup_saved" ? tryT(`event.saved.${p.kind}`, p) : e.code ? tryT(`event.${e.code}`, p) : undefined;
  return text ?? e.message;
}

export function Activity() {
  const { t, tryT, errorText } = useI18n();
  const f = useFormat();
  const { data, error } = useLoad(api.events, [], 5000);
  if (error) return <div className="callout error">{errorText(error)}</div>;
  if (!data) return <p className="muted">{t("common.loading")}</p>;
  return (
    <section className="panel">
      <h2>{t("activity.title")}</h2>
      {data.length === 0 && <p className="muted">{t("activity.empty")}</p>}
      <ul className="events">
        {data.map((e, i) => (
          <li key={i} className={`event ${e.kind}`}>
            <span className="muted small">{f.dateTime(e.time)}</span>
            <a href={`#/world/${encodeURIComponent(e.worldId)}`}>{e.world || e.worldId}</a>
            <span>{eventText(e, tryT)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
