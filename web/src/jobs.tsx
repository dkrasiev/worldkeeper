import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { api, ApiError, token, type Job } from "./api";
import { useFormat } from "./format";
import { useI18n } from "./i18n";

interface Jobs {
  jobs: Job[];
  /** Shows a job the UI just started without waiting for the next poll. */
  track: (job: Job) => void;
}

const JobsContext = createContext<Jobs | null>(null);

// JobsProvider polls the server's jobs: every second while one runs, so
// progress moves, and every few seconds otherwise, to notice automatic
// backups. Jobs live on the server, so leaving a page never cancels one.
export function JobsProvider({ children }: { children: ReactNode }) {
  const [jobs, setJobs] = useState<Job[]>([]);
  const running = jobs.some((j) => j.state === "running");

  useEffect(() => {
    if (!token) return;
    let stopped = false;
    const tick = () =>
      api.jobs().then(
        (js) => !stopped && setJobs(js),
        () => {},
      );
    tick();
    const timer = setInterval(tick, running ? 1000 : 5000);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [running]);

  const track = useCallback((job: Job) => setJobs((js) => [job, ...js.filter((j) => j.id !== job.id)]), []);
  return <JobsContext.Provider value={{ jobs, track }}>{children}</JobsContext.Provider>;
}

export function useJobs(): Jobs {
  const ctx = useContext(JobsContext);
  if (!ctx) throw new Error("useJobs outside JobsProvider");
  return ctx;
}

// useWorldJob returns the job running for a world and calls onFinish once
// for each job of that world this page saw running (or started) when it ends.
export function useWorldJob(worldId: string, onFinish: (job: Job) => void) {
  const { jobs, track } = useJobs();
  const watching = useRef(new Set<string>());
  const onFinishRef = useRef(onFinish);
  onFinishRef.current = onFinish;

  useEffect(() => {
    for (const j of jobs) {
      if (j.worldId !== worldId) continue;
      if (j.state === "running") watching.current.add(j.id);
      else if (watching.current.delete(j.id)) onFinishRef.current(j);
    }
  }, [jobs, worldId]);

  const start = useCallback(
    (job: Job) => {
      watching.current.add(job.id);
      track(job);
    },
    [track],
  );
  return { job: jobs.find((j) => j.worldId === worldId && j.state === "running"), track: start };
}

/** The error of a failed job, in the form errorText understands. */
export function jobError(job: Job): ApiError {
  return new ApiError(0, job.error?.error ?? "internal", job.error?.message ?? "");
}

const percent = (job: Job) => (job.total > 0 ? Math.min(100, Math.floor((job.done / job.total) * 100)) : undefined);

function usePhaseLabel() {
  const { t } = useI18n();
  return (job: Job) =>
    ({ backup: t("job.backup"), safety: t("job.safety"), restore: t("job.restore") })[job.phase];
}

/** Progress of a world's running job, shown on its pages. */
export function JobProgress({ job }: { job: Job }) {
  const { t } = useI18n();
  const f = useFormat();
  const phase = usePhaseLabel();
  const pct = percent(job);
  return (
    <div className="callout job" role="status">
      <div className="job-line">
        <strong>{phase(job)}</strong>
        {pct !== undefined && (
          <span className="muted small">
            {t("job.progress", { done: f.bytes(job.done), total: f.bytes(job.total) })} · {pct}%
          </span>
        )}
      </div>
      <progress max={100} value={pct} aria-label={phase(job)} />
      <div className="muted small">{t("job.leaveHint")}</div>
    </div>
  );
}

/** Running jobs in the top bar, so they stay visible on every page. */
export function JobsIndicator() {
  const { t } = useI18n();
  const { jobs } = useJobs();
  const running = jobs.filter((j) => j.state === "running");
  if (running.length === 0) return null;
  return (
    <div className="job-chips">
      {running.map((j) => {
        const pct = percent(j);
        return (
          <a key={j.id} className="job-chip" href={`#/world/${encodeURIComponent(j.worldId)}`}>
            <span className="spinner" aria-hidden />
            {t(j.kind === "backup" ? "job.chipBackup" : "job.chipRestore", { world: j.world })}
            {pct !== undefined && <span className="muted">{pct}%</span>}
          </a>
        );
      })}
    </div>
  );
}
