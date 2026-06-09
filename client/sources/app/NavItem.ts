import { LayoutDashboard, ArrowLeftRight, Activity, Network, Settings, FileCode, type LucideIcon } from "lucide-react";

export type AppView = "dashboard" | "rules" | "activity" | "connections" | "settings" | "api-docs";

export type NavItem = { id: AppView; label: string; Icon: LucideIcon };

export const NAV_ITEMS: NavItem[] = [
  { id: "dashboard", label: "Dashboard", Icon: LayoutDashboard },
  { id: "rules", label: "Forward Rules", Icon: ArrowLeftRight },
  { id: "activity", label: "Activity", Icon: Activity },
  { id: "connections", label: "Connections", Icon: Network },
  { id: "settings", label: "Settings", Icon: Settings },
  { id: "api-docs", label: "API Docs", Icon: FileCode },
];
