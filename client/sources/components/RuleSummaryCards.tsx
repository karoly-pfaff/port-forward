import type { ReactElement } from "react";
import type { ForwardRuleResponse, ForwardStatus } from "@portier/shared";
import { StatCard } from "./StatCard.js";

interface RuleSummaryCardsProps {
  rules: ForwardRuleResponse[];
  statusMap: Map<string, ForwardStatus>;
  /** Override description for the Total Rules card. Defaults to "N TCP · N UDP". */
  totalDesc?: string;
}

export function RuleSummaryCards({ rules, statusMap, totalDesc }: RuleSummaryCardsProps): ReactElement {
  const runningCount = rules.filter((r) => statusMap.get(r.id)?.running).length;
  const errorCount = rules.filter((r) => !!statusMap.get(r.id)?.lastError).length;
  const stoppedCount = rules.length - runningCount;
  const tcpCount = rules.filter((r) => r.protocol === "tcp").length;
  const udpCount = rules.filter((r) => r.protocol === "udp").length;

  return (
    <div className="stat-cards" aria-label="Rule summary">
      <StatCard
        variant="total"
        value={rules.length}
        label="Total Rules"
        desc={totalDesc ?? `${tcpCount} TCP · ${udpCount} UDP`}
      />
      <StatCard variant="running" value={runningCount} label="Running" desc="Currently forwarding" />
      <StatCard variant="stopped" value={stoppedCount} label="Stopped" desc="Not forwarding" />
      <StatCard variant="error" value={errorCount} label="Error" desc="Needs attention" />
    </div>
  );
}
