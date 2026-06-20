import { useState } from "react";

export interface ClipboardCopy {
  copyToClipboard: (text: string, key: string) => Promise<void>;
  copyLabel: (key: string) => string;
}

// useClipboardCopy provides copy-to-clipboard with transient per-key feedback
// ("Copied!" / "Failed", reverting after 2s). A missing clipboard API is treated
// as a failure, matching the previous inline behaviour.
export function useClipboardCopy(): ClipboardCopy {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  async function copyToClipboard(text: string, key: string): Promise<void> {
    try {
      if (!navigator.clipboard) throw new Error("unavailable");
      await navigator.clipboard.writeText(text);
      setCopiedKey(key);
      setTimeout(() => setCopiedKey((k) => (k === key ? null : k)), 2000);
    } catch {
      setCopiedKey(`${key}:fail`);
      /* v8 ignore next -- stale-timer guard: the failed-key label only auto-clears if still unchanged; the else arm (a newer copy already replaced it) is defensive */
      setTimeout(() => setCopiedKey((k) => (k === `${key}:fail` ? null : k)), 2000);
    }
  }

  function copyLabel(key: string): string {
    if (copiedKey === key) return "Copied!";
    if (copiedKey === `${key}:fail`) return "Failed";
    return "Copy";
  }

  return { copyToClipboard, copyLabel };
}
