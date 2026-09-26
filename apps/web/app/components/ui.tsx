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
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <path d="M5 19 19 5M5 5h14v14" />
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
