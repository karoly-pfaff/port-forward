import type { ReactElement } from "react";
import type { RuntimeRecovery } from "@portier/shared";

/**
 * Operator-facing banner shown only when the runtime reports an active config
 * recovery state (v1.17 `GET /api/runtime` → `recovery.active`). It explains that
 * Portier started without the saved configuration, that the UI/API remain usable,
 * whether writes are blocked, and where the original file was preserved.
 *
 * Per-rule autostart/duplicate failures are NOT global recovery — they surface in
 * the rule's status/health, so this banner stays hidden for them. Returns null
 * (no banner) when recovery is inactive or unavailable.
 */
export function RecoveryBanner({
  recovery,
}: {
  recovery: RuntimeRecovery | null | undefined;
}): ReactElement | null {
  if (!recovery?.active) {
    return null;
  }

  return (
    <section className="recovery-banner" role="alert" aria-live="polite">
      <p>
        <strong>Configuration recovery mode.</strong>{" "}
        {recovery.message ?? "Portier started without your saved configuration."}
      </p>
      {recovery.writesBlocked && (
        <p>
          Rule changes are blocked until the configuration is repaired. The management UI and API
          remain available — import or save a valid configuration to continue.
        </p>
      )}
      {recovery.quarantinePath && (
        <p>
          Your original configuration was preserved at <code>{recovery.quarantinePath}</code>.
        </p>
      )}
    </section>
  );
}
