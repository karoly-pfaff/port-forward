import type { ReactElement, ReactNode } from "react";

// SettingsSection is the shared title + body wrapper used by every settings
// panel. It renders exactly the `.settings-section` / `.settings-section-title`
// structure the panels previously spelled inline, so the DOM is unchanged.
export function SettingsSection({
  title,
  children
}: {
  title: string;
  children: ReactNode;
}): ReactElement {
  return (
    <div className="settings-section">
      <div className="settings-section-title">{title}</div>
      {children}
    </div>
  );
}
