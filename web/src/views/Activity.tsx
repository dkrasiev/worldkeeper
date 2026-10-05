import { api } from "../api";
import { dateTime } from "../format";
import { useLoad } from "../hooks";

export function Activity() {
  const { data, error } = useLoad(api.events, [], 5000);
  if (error) return <div className="callout error">{error.message}</div>;
  if (!data) return <p className="muted">Loading…</p>;
  return (
    <section className="panel">
      <h2>Activity</h2>
      {data.length === 0 && <p className="muted">Nothing yet since Worldkeeper started.</p>}
      <ul className="events">
        {data.map((e, i) => (
          <li key={i} className={`event ${e.kind}`}>
            <span className="muted small">{dateTime(e.time)}</span>
            <a href={`#/world/${encodeURIComponent(e.worldId)}`}>{e.world || e.worldId}</a>
            <span>{e.message}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
