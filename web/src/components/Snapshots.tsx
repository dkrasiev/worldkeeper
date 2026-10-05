import { useState } from "react";
import type { RestoreMode, Snapshot } from "../api";
import { useFormat } from "../format";
import { useI18n } from "../i18n";
import { Modal } from "./ui";

// Safety snapshots store only what they preceded ("“Label”" or a date);
// labels written by older versions carried an English prefix.
const legacyPrefix = /^(Before loading |before restoring )/;

export function useSnapTitle() {
  const { t } = useI18n();
  const f = useFormat();
  return (s: Snapshot) => {
    if (s.kind === "pre-restore" && s.label) return t("snap.before", { name: s.label.replace(legacyPrefix, "") });
    return s.label || f.dateTime(s.createdAt);
  };
}


export function LoadDialog({
  title,
  missing,
  inUse,
  onClose,
  onLoad,
}: {
  title: string;
  missing: boolean;
  inUse: boolean;
  onClose: () => void;
  onLoad: (mode: RestoreMode) => void;
}) {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const go = (mode: RestoreMode) => {
    setBusy(true);
    onLoad(mode);
  };
  return (
    <Modal title={t("load.title", { name: title })} onClose={onClose}>
      {missing ? (
        <p>{t("load.missingHint")}</p>
      ) : (
        <>
          {inUse && <div className="callout warn">{t("load.exitFirst")}</div>}
          <div className="choice">
            <button className="choice-btn" disabled={busy || inUse} onClick={() => go("replace")}>
              <strong>{t("load.replace")}</strong>
              <span>{t("load.replaceHint")}</span>
            </button>
            <button className="choice-btn" disabled={busy} onClick={() => go("copy")}>
              <strong>{t("load.copy")}</strong>
              <span>{t("load.copyHint")}</span>
            </button>
          </div>
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
