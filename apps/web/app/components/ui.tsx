"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { transportModes, type TransportMode } from "../../lib/identity-storage";

export const transportLabels: Record<TransportMode, string> = {
  walking: "Walking",
  bicycle: "Bicycle",
  car: "Car",
  bus: "Bus",
  train: "Train",
  boat: "Boat",
  airplane: "Airplane",
};

export function Brand() {
  return (
    <a className="brand" href="/" aria-label="KeepUp home">
      keep<span>up</span>
      <span className="brand-mark" aria-hidden="true">
        <svg viewBox="0 0 128 128">
          <rect width="128" height="128" fill="#173023" />
          <rect x="5" y="5" width="118" height="118" rx="34" fill="#173023" stroke="#3f7652" strokeWidth="6" />
          <path d="M42 31v66" fill="none" stroke="#f8fafc" strokeWidth="14" strokeLinecap="round" />
          <path d="M87 96C72 82 64 69 49 65C64 63 75 52 83 39" fill="none" stroke="#22c55e" strokeWidth="13" strokeLinecap="round" strokeLinejoin="round" />
          <path d="m78 35 14-4-2 15" fill="none" stroke="#22c55e" strokeWidth="9" strokeLinecap="round" strokeLinejoin="round" />
          <circle cx="81" cy="90" r="7" fill="#f8fafc" stroke="#173023" strokeWidth="4" />
          <circle cx="73" cy="52" r="7" fill="#f8fafc" stroke="#173023" strokeWidth="4" />
        </svg>
      </span>
    </a>
  );
}

export function TransportField({
  value,
  onChange,
}: {
  value: TransportMode;
  onChange: (value: TransportMode) => void;
}) {
  return (
    <label className="field">
      <span>Transport</span>
      <select
        name="transportMode"
        value={value}
        onChange={(event) => onChange(event.target.value as TransportMode)}
      >
        {transportModes.map((mode) => (
          <option key={mode} value={mode}>
            {transportLabels[mode]}
          </option>
        ))}
      </select>
    </label>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const labels: Record<string, string> = {
    tracking: "Sharing",
    stale: "Stale",
    spectating: "Spectating",
    offline: "Offline",
    left: "Left",
  };
  return (
    <span className={`status-badge status-${status}`}>
      <span aria-hidden="true" className="status-dot" />
      {labels[status] ?? status}
    </span>
  );
}

// Native modal dialogs keep keyboard focus inside and make the page inert.
export function Modal({
  labelledBy,
  describedBy,
  onClose,
  children,
}: {
  labelledBy: string;
  describedBy?: string;
  onClose?: () => void;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    const previous = document.activeElement;
    dialog?.showModal();
    dialog?.querySelector<HTMLElement>("[data-initial-focus]")?.focus();
    return () => {
      dialog?.close();
      if (previous instanceof HTMLElement && previous.isConnected)
        previous.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className="recovery-dialog"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      onCancel={(event) => {
        event.preventDefault();
        onClose?.();
      }}
    >
      {children}
    </dialog>
  );
}
