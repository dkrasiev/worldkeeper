import { useEffect, useRef, useState, type ReactNode } from "react";
import { api, type GameMode, type SnapshotKind } from "../api";
import { useI18n } from "../i18n";

export function WorldIcon({ id, hasIcon, size = 64 }: { id?: string; hasIcon?: boolean; size?: number }) {
  const [broken, setBroken] = useState(false);
  return hasIcon && id && !broken ? (
    <img className="world-icon" src={api.iconUrl(id)} width={size} height={size} alt="" onError={() => setBroken(true)} />
  ) : (
    <span className="world-icon placeholder" style={{ width: size, height: size }} aria-hidden />
  );
}

export function ModeBadge({ mode, hardcore }: { mode: GameMode; hardcore: boolean }) {
  const { t } = useI18n();
  if (hardcore) return <span className="badge hardcore">{t("mode.hardcore")}</span>;
  if (!mode) return null;
  return <span className={`badge mode-${mode}`}>{t(`mode.${mode}`)}</span>;
}

export function KindBadge({ kind }: { kind: SnapshotKind }) {
  const { t } = useI18n();
  return <span className={`badge kind-${kind}`}>{t(`kind.${kind}`)}</span>;
}

export function Modal({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog ref={ref} className="modal" onClose={onClose} onCancel={onClose}>
      <h3>{title}</h3>
      {children}
    </dialog>
  );
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="field">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}
