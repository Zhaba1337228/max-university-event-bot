"use client";

// Lightweight inline toast — no context needed.
// Usage: <Toast message="..." kind="success" onDismiss={() => …} />
// Or: <Toast message="..." kind="error" />

import clsx from "clsx";
import { useEffect, useState } from "react";
import { IconCheckCircle, IconXCircle, IconInfo, IconX } from "./icons";

export type ToastKind = "success" | "error" | "info";

type Props = {
  message: string;
  kind?: ToastKind;
  onDismiss?: () => void;
  autoDismiss?: number; // ms, default 4000
};

const styles: Record<ToastKind, string> = {
  success: "border-success/40 bg-success/10 text-success",
  error: "border-danger/40 bg-danger/10 text-danger",
  info: "border-accent/40 bg-accent/10 text-accent",
};

const Icon = ({ kind }: { kind: ToastKind }) => {
  const cls = "w-5 h-5 shrink-0";
  if (kind === "success") return <IconCheckCircle className={cls} />;
  if (kind === "error") return <IconXCircle className={cls} />;
  return <IconInfo className={cls} />;
};

export function Toast({ message, kind = "info", onDismiss, autoDismiss = 4000 }: Props) {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    if (!autoDismiss) return;
    const t = setTimeout(() => {
      setVisible(false);
      onDismiss?.();
    }, autoDismiss);
    return () => clearTimeout(t);
  }, [autoDismiss, onDismiss]);

  if (!visible) return null;

  return (
    <div
      className={clsx(
        "flex items-start gap-2.5 rounded-lg border px-4 py-3 text-sm shadow-elevated",
        "animate-in fade-in slide-in-from-top-2 duration-200",
        styles[kind],
      )}
      role="alert"
    >
      <Icon kind={kind} />
      <span className="flex-1 leading-snug">{message}</span>
      {onDismiss && (
        <button
          type="button"
          onClick={() => { setVisible(false); onDismiss(); }}
          className="ml-1 shrink-0 opacity-60 hover:opacity-100 transition-opacity"
          aria-label="Закрыть"
        >
          <IconX size={14} />
        </button>
      )}
    </div>
  );
}
