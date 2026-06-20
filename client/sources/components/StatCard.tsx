import type { ReactElement } from "react";

type StatCardVariant = "total" | "running" | "stopped" | "error";

const ICONS: Record<StatCardVariant, string> = {
  total: "≡",
  running: "▶",
  stopped: "■",
  error: "⚠",
};

interface StatCardProps {
  variant: StatCardVariant;
  value: number;
  label: string;
  desc: string;
  /** When set, the card becomes a button that triggers this on click. */
  onClick?: () => void;
}

export function StatCard({ variant, value, label, desc, onClick }: StatCardProps): ReactElement {
  const inner = (
    <>
      <div className="stat-card-left">
        <div className={`stat-card-icon stat-card-icon--${variant}`} aria-hidden="true">
          {ICONS[variant]}
        </div>
        <div className="stat-card-value">{value}</div>
      </div>
      <div className="stat-card-right">
        <div className="stat-card-label">{label}</div>
        <div className="stat-card-desc">{desc}</div>
      </div>
    </>
  );

  if (onClick) {
    return (
      <button
        type="button"
        className="stat-card stat-card--clickable"
        onClick={onClick}
        aria-label={`${label}: ${value}. ${desc}`}
      >
        {inner}
      </button>
    );
  }

  return <div className="stat-card">{inner}</div>;
}
