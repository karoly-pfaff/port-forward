import { LayoutDashboard, ArrowLeftRight, Activity, Network, Settings, type LucideIcon } from "lucide-react";

export type AppView = "dashboard" | "rules" | "activity" | "connections" | "settings" | "api-docs";

export type NavItem = { id: AppView; label: string; Icon: LucideIcon };

// Sidebar navigation. "api-docs" is intentionally absent here — it is reached from the
// header (TopHeader), not the left menu.
export const NAV_ITEMS: NavItem[] = [
  { id: "dashboard", label: "Dashboard", Icon: LayoutDashboard },
  { id: "rules", label: "Forward Rules", Icon: ArrowLeftRight },
  { id: "activity", label: "Activity", Icon: Activity },
  { id: "connections", label: "Live Connections", Icon: Network },
  { id: "settings", label: "Settings", Icon: Settings },
];
