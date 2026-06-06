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
}

export function StatCard({ variant, value, label, desc }: StatCardProps): ReactElement {
  return (
    <div className="stat-card">
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
    </div>
  );
}
