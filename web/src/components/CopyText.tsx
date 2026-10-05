import { useEffect, useRef, useState } from "react";

const isMac = /Mac|iPhone|iPad/.test(navigator.userAgent);

// CopyText shows a value with a Copy button. The Clipboard API can refuse
// (no secure context, tab not focused, permission denied), so on failure
// the text is selected instead and the user is told to press the shortcut.
export function CopyText({ text }: { text: string }) {
  const [state, setState] = useState<"idle" | "copied" | "manual">("idle");
  const textRef = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (state === "idle") return;
    const t = setTimeout(() => setState("idle"), state === "copied" ? 2000 : 5000);
    return () => clearTimeout(t);
  }, [state]);

  const selectText = () => {
    const el = textRef.current;
    const sel = window.getSelection();
    if (!el || !sel) return;
    const range = document.createRange();
    range.selectNodeContents(el);
    sel.removeAllRanges();
    sel.addRange(range);
  };

  const copy = async () => {
    try {
      if (!navigator.clipboard) throw new Error("Clipboard API unavailable");
      await navigator.clipboard.writeText(text);
      setState("copied");
    } catch {
      selectText();
      setState("manual");
    }
  };

  return (
    <span className="copy-text">
      <span ref={textRef} className="mono selectable">
        {text}
      </span>
      <button type="button" className="link small" onClick={copy}>
        Copy
      </button>
      <span className={`copy-status small ${state}`} role="status" aria-live="polite">
        {state === "copied" && "Copied ✓"}
        {state === "manual" && `Press ${isMac ? "⌘C" : "Ctrl+C"} to copy`}
      </span>
    </span>
  );
}
