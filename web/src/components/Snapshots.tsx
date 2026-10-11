import { useState } from "react";
import type { RestoreMode, Snapshot } from "../api";
import { useFormat } from "../format";
import { useI18n } from "../i18n";
import { LocationPicker } from "./LocationPicker";
import { Modal } from "./ui";

// Safety snapshots store only what they preceded ("“Label”" or a date);
// labels written by older versions carried an English prefix.
const legacyPrefix = /^(Before loading |before restoring )/;

export function useSnapTitle() {
  const { t } = useI18n();
  const f = useFormat();
  return (s: Snapshot) => {
    if (s.kind === "pre-delete") return t("snap.beforeDelete");
    if (s.kind === "pre-move") return t("snap.beforeMove");
    if (s.kind === "pre-restore" && s.label) return t("snap.before", { name: s.label.replace(legacyPrefix, "") });
    return s.label || f.dateTime(s.createdAt);
  };
}


// LoadDialog asks how to load a save. A copy, or a world that is not on
// this computer, can go into any saves folder; savesDir is passed only when
// the user picked one.
export function LoadDialog({
  title,
  missing,
  inUse,
  savesDir,
  onClose,
  onLoad,
}: {
  title: string;
  missing: boolean;
  inUse: boolean;
  /** The local world's saves folder: where a copy goes by default. */
  savesDir?: string;
  onClose: () => void;
  onLoad: (mode: RestoreMode, savesDir?: string) => void;
}) {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const [target, setTarget] = useState(missing ? "" : (savesDir ?? ""));
  const go = (mode: RestoreMode) => {
    setBusy(true);
    const to = target.trim();
    // A world without a local folder has nothing to replace: picking a
    // folder makes it a plain restore there.
    onLoad(missing && to ? "copy" : mode, to || undefined);
  };
  return (
    <Modal title={t("load.title", { name: title })} onClose={onClose}>
      {missing ? (
        <>
          <p>{t("load.missingHint")}</p>
          <LocationPicker label={t("load.restoreTo")} value={target} onChange={setTarget} auto={t("location.auto")} />
        </>
      ) : (
        <>
          {inUse && <div className="callout warn">{t("load.exitFirst")}</div>}
          <div className="choice">
            <button className="choice-btn" disabled={busy || inUse} onClick={() => go("replace")}>
              <strong>{t("load.replace")}</strong>
              <span>{t("load.replaceHint")}</span>
            </button>
            <button className="choice-btn" disabled={busy || !target.trim()} onClick={() => go("copy")}>
              <strong>{t("load.copy")}</strong>
              <span>{t("load.copyHint")}</span>
            </button>
          </div>
          <LocationPicker label={t("load.copyTo")} value={target} onChange={setTarget} />
        </>
      )}
      <div className="actions">
        <button onClick={onClose}>{t("common.cancel")}</button>
        {missing && (
          <button className="primary" disabled={busy} onClick={() => go("replace")}>
            {busy ? t("load.restoring") : t("load.restore")}
          </button>
        )}
      </div>
    </Modal>
  );
}


export function DeleteDialog({ title, onClose, onDelete }: { title: string; onClose: () => void; onDelete: () => void }) {
  const { t } = useI18n();
  return (
    <Modal title={t("detail.deleteTitle")} onClose={onClose}>
      <p>{t("detail.deleteBody", { name: title })}</p>
      <div className="actions">
        <button onClick={onClose}>{t("common.cancel")}</button>
        <button className="danger" onClick={onDelete}>
          {t("common.delete")}
        </button>
      </div>
    </Modal>
  );
}
