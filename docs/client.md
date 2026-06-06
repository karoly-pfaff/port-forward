# Client

Portier's client is a React/Vite UI for managing local TCP and UDP forwarding rules.

## Current UI Structure

```
client/sources/
  app/
    App.tsx               — orchestration: view state, data loading, action handlers
    nav.ts                — AppView type, NAV_ITEMS constant
    Sidebar.tsx           — sidebar nav + running status footer
    TopHeader.tsx         — header: brand, subtitle, Settings and API Docs shortcuts
  api/
    portierApi.ts         — all fetch wrappers for the Portier REST API
  components/
    AdvisoryList.tsx      — renders PortAdvisory[] messages (compact, for table rows)
    StatCard.tsx          — single summary stat card (icon, value, label, desc)
    RuleSummaryCards.tsx  — 4-card grid (Total/Running/Stopped/Error) from rules + statusMap
  features/
    forwards/
      RuleForm.ts             — RuleFormState type, emptyForm, ruleToForm, formToPayload
      ForwardRuleForm.tsx     — add/edit form with field-level errors and advisory cards
      ForwardRuleList.tsx     — rules table with search/filter, per-rule busy state, inline delete confirm, drag-to-reorder
      ForwardStatusBadge.tsx  — running/stopped/error badge with optional error click
    activity/
      ActivityLogView.tsx     — event list, severity filter, limit selector, auto-refresh
    dashboard/
      DashboardView.tsx       — stat cards (via RuleSummaryCards), top active rules, recent activity
    settings/
      SettingsView.tsx        — management endpoint info, port range, config export/import, about
    api-docs/
      ApiDocsView.tsx         — client-side REST API reference page
  utils/
    format.ts             — formatBytes, formatTimestamp, formatUdpModeLabel
  styles/
    styles.css            — global stylesheet, dark developer-tool theme
  main.tsx                — mounts React root, imports styles
```

Test files are colocated with their source files.

### Component Naming Convention

React component and view files under `client/sources/` use **CamelCase** filenames (e.g., `ForwardRuleList.tsx`, `StatCard.tsx`). Non-component files (utilities, config, types) use the repo convention (e.g., `format.ts`, `nav.ts`, `portierApi.ts`).

## App Shell Layout

The App renders a full-screen layout:

```
┌─────────────────────────────────────────────────────┐
│  TopHeader (full width)                             │
├───────────┬─────────────────────────────┬───────────┤
│  Sidebar  │  main-content               │  drawer   │
│           │  view-specific content      │  (open    │
│  nav      │                             │  when     │
│           │                             │  editing/ │
│  footer   │                             │  adding)  │
└───────────┴─────────────────────────────┴───────────┘
```

- **TopHeader**: brand logo/title, subtitle, API Docs shortcut, Settings button, hamburger menu (mobile)
- **Sidebar**: nav buttons (all five functional), running status footer
- **main-content**: error/unavailable banners, active view content
- **drawer**: right-side panel rendered only when `showForm` is true (add/edit rule)

### Mobile Sidebar

At `≤700px` the sidebar is hidden. A hamburger button (`☰`) appears in the header with `aria-label="Open navigation menu"`. Clicking it opens the sidebar as an overlay with `.sidebar--mobile-open`. A `.sidebar-backdrop` backdrop closes it on click. Pressing Escape also closes it. Clicking any nav item closes it.

## View State

`App.tsx` manages a `view: "dashboard" | "rules" | "activity" | "settings" | "api-docs"` state variable. Default is `"rules"`.

All five sidebar items are functional `button.nav-item` elements. Clicking navigates to the view and clears the edit drawer (except `"rules"` preserves the drawer state).

`handleNavClick(next)` sets the view and sets `mobileSidebarOpen = false`.

## Drawer State

`App.tsx` manages `showForm: boolean` alongside `editingRuleId: string | null`.

- `showForm = false` → drawer hidden
- `showForm = true` + `editingRuleId = null` → Add mode
- `showForm = true` + `editingRuleId = "some-id"` → Edit mode

Opening:
- `handleEditRule(rule)` → sets both `editingRuleId` and `showForm = true`
- `handleAddRule()` → clears `editingRuleId`, sets `showForm = true`, also navigates to `"rules"` view

Closing:
- `handleCancel()` → clears both
- Successful save → clears both
- Delete closes drawer if the deleted rule was being edited

## Dashboard View (`DashboardView`)

Props: `rules`, `statusMap`, `recentActivity`, `onAddRule`, `onGoToActivity`, `onGoToRules`.

Shows:
- Stat cards: Total Rules (with TCP/UDP breakdown), Running, Stopped, Error
- Top Rules by Traffic: up to 5 rules sorted by `bytesIn + bytesOut`; empty state when no traffic
- Recent Activity: last 5 events from `recentActivity` prop; empty state when none
- Quick actions: `+ Add Rule`, `View All` (rules), `View All` (activity)

`App.tsx` fetches the last 5 activity events on initial load via `fetchActivity({ limit: 5 })`.

## Rules View (`ForwardRuleList` + stat cards)

The rules view renders the four stat cards (Total/Running/Stopped/Error) above the `ForwardRuleList`. The stat cards in the rules view use the same data as the dashboard.

### Rule List Features

- **Search**: local filter by name, host, port, or protocol
- **Status filter**: All / Running / Stopped / Error
- **Auto-refresh toggle**: checkbox + optional interval select (2s/5s/10s/30s)
- **Autostart column**: Yes/No
- **Selected row**: `row-selected` class applied when `editingRuleId` matches the row
- **Move Up/Down buttons**: shown only when no search/filter is active; call `onMoveRule` prop
- **Add Rule button**: calls `onAddRule` prop

### Rule Ordering

`onMoveRule(rule, "up" | "down")` in `App.tsx` builds a new ID order by swapping adjacent elements, then calls `reorderForwardRules(newOrder)` which hits `POST /api/forwards/reorder`. Running rules are not restarted. Move buttons are hidden when search or status filter is active (to avoid ordering confusion with filtered views).

## Settings View (`SettingsView`)

Props: `onRulesUpdated(rules: ForwardRuleResponse[])`.

Sections:
1. **Management Endpoint**: shows `127.0.0.1:47831`; warns about LAN exposure on `0.0.0.0`
2. **Recommended Forward Port Range**: shows `48000–48999`; explains ports outside range are warned but not blocked
3. **Export Config**: download button; calls `exportConfig()` → `GET /api/config/export`; triggers browser download of `portier-rules-YYYY-MM-DD.json`
4. **Import Config**: file picker (JSON), parses client-side, shows preview (count, TCP/UDP/enabled breakdown), mode selector (Merge/Replace), Replace mode has a two-step confirm; calls `importConfig(config, mode)` → `POST /api/config/import`; calls `onRulesUpdated` on success
5. **About Portier**: version, management default, notes on in-memory activity log

## API Docs View (`ApiDocsView`)

No props. Static list of all REST endpoints rendered client-side. Entries:
- `GET /api/forwards`
- `POST /api/forwards`
- `PATCH /api/forwards/:id`
- `DELETE /api/forwards/:id`
- `POST /api/forwards/:id/start`
- `POST /api/forwards/:id/stop`
- `POST /api/forwards/reorder`
- `GET /api/status`
- `GET /api/activity`
- `GET /api/config/export`
- `POST /api/config/import`
- `GET /api/ports/advisory`

Each entry shows: method badge (colour-coded), path, purpose, params (if any), response shape.

## Activity Log View (`ActivityLogView`)

No props. Fetches from `GET /api/activity` on mount and optionally on a timer. Supports severity filter and limit selector.

## Form Structure

`ForwardRuleForm` renders three sections used inside the drawer wrapper in `App.tsx`:

1. **drawer-header** — title (h2 "Add Rule" / "Edit Rule"), protocol badge + rule name when editing, close (✕) button
2. **drawer-body** — form errors panel, form fields, advisory cards
3. **drawer-footer** — Cancel (edit mode only), Delete Rule (edit mode + onDelete provided), Save Changes / Add Rule

The submit button is outside the `<form>` element and connects via `form="rule-form"` attribute.

## Form Reset

`ForwardRuleForm` initializes its state from `editingRule` once on mount. `App.tsx` controls reset by changing the `key` prop — `key={editingRuleId ?? "new"}`. When the key changes, the component remounts and reinitializes cleanly.

## Validation

Client-side validation uses `validateForwardRule` from `@portier/shared`. Port fields show inline field-level errors when the value is non-empty and out of range. Form-level validation errors appear in a `role="alert"` panel inside the drawer body.

Port advisories use `getPortAdvisories` from `@portier/shared` and render as styled advisory cards in the drawer body.

## Status Badge States

`ForwardStatusBadge` shows three states:

| Condition                      | Badge text | Badge class |
|-------------------------------|------------|-------------|
| `status.running === true`      | Running    | `.running`  |
| `!running && !lastError`       | Stopped    | `.stopped`  |
| `!running && lastError`        | Error      | `.error`    |

Always shows "Autostart on/off" and `lastError` text below the badge.

## Server Communication

The client talks to the server through relative `/api` routes. During development, Vite proxies `/api` to `http://127.0.0.1:47831`.

`App.tsx` loads rules, statuses, and recent activity on startup and after each mutation:
- `GET /api/forwards`
- `GET /api/status`
- `GET /api/activity?limit=5` (startup only)

All fetch wrappers live in `client/sources/api/portierApi.ts`.

## Styling

`styles.css` uses CSS custom properties for the entire design system:

- Dark backgrounds: `--bg-base` (#0d1117), `--bg-surface` (#161b22), `--bg-raised` (#1c2128)
- Borders: `--border-default` (#30363d)
- Text: `--text-primary` / `--text-secondary` / `--text-muted`
- Status colors: `--green`, `--amber`, `--red` with matching `--*-bg` and `--*-border`
- Protocol badge: blue (TCP) and purple (UDP)
- Monospace font for endpoint cells

Layout dimensions: `--sidebar-width: 196px`, `--drawer-width: 360px`.

Responsive breakpoints:
- `≤960px`: stat cards go 2-column
- `≤700px`: sidebar hidden, hamburger button visible, drawer becomes full-overlay

## Current States

- **Loading**: "Loading rules…" shown inside the table wrapper on first load
- **Empty**: "No forwarding rules yet. Click + Add Rule to create one." when rules list is empty
- **No search match**: "No rules match the current filter." when filter/search produces empty result
- **Server unavailable**: `.server-unavailable` banner with host:port hint
- **Action error**: `.errors` banner at top of `main-content`
- **Save error**: Error panel inside the drawer body
- **Rule error state**: Row gets a red left border; badge shows "Error"; `lastError` shown below badge
- **Selected row**: Blue left border + blue-tinted background
- **Running / error count**: Shown in rule-list-section header

## Auto-Refresh

`App.tsx` owns auto-refresh state:

- `autoRefresh: boolean` — toggle on/off
- `autoRefreshInterval: number` — seconds (2/5/10/30, default 5)
- `refreshInFlightRef` — prevents overlapping background fetches

Props `autoRefresh`, `autoRefreshInterval`, `onToggleAutoRefresh`, `onChangeAutoRefreshInterval` are threaded through to `ForwardRuleList`.

## Escape Key

A `useEffect` on `[showForm]` attaches a keydown listener when the drawer is open. Pressing Escape calls `handleCancel()` via `handleCancelRef`. A separate `useEffect` on `[mobileSidebarOpen]` closes the mobile sidebar on Escape.

## Favicon

`client/index.html` references `/brand/portier-logo-transparent.png` as the page favicon. The same image is used as the sidebar brand logo with `alt="Portier logo"`.

## Constraints

- No new UI framework
- No Tailwind
- No icon library
- Keep the management UI/API localhost by default
- Do not change API contracts without updating `docs/api-contract.md`

## Known Remaining Issues

- `refreshAll` after actions does a full reload; no optimistic updates
- `lastError` in the status cell truncates via CSS ellipsis; full text available via `title` attribute
- Activity log resets on server restart (in-memory only)
- Recent activity on Dashboard is fetched once on load; it does not auto-refresh

## Validation After UI Changes

```powershell
npm run check
npm run build
```

## Client Test Coverage

Tests use Vitest, jsdom, and React Testing Library.

```powershell
npm run test:client        # client tests only
npm run test               # shared + server + client
```

Coverage (99 tests across 8 files):
- `ForwardStatusBadge` (8): running/stopped/error text, last error, accessible text
- `ForwardRuleList` (20): empty state, loading, rule render, stats, autostart column, delete confirm, start/stop, per-rule busy, Add Rule, row-selected, auto-refresh toggle, interval select, toggle callback
- `ForwardRuleForm` (17): add/edit mode, invalid port blocking, UDP mode toggle, LAN exposure advisory, common port advisory, validation errors, input preservation, save error, Cancel/Delete Rule
- `ActivityLogView` (8): renders events, severity filter, empty state, auto-refresh toggle
- `DashboardView` (8): heading, stat cards, TCP/UDP breakdown, no-traffic state, top rules, no-activity state, activity events, Add Rule button
- `SettingsView` (11): management endpoint, port range, export button, import file picker, about/version, localhost-only note, atomic import note, import merge/replace mode, replace confirm flow
- `ApiDocsView` (10): API Reference heading, endpoint listing (forwards, reorder, status, activity, config export/import, port advisory), method badges, reorder note
- `App` (17): brand name, logo alt text, server unavailable, loaded rules with running count, error count, API Docs nav, all nav items functional, Activity nav switch, Dashboard renders summaries, mobile menu opens sidebar, nav item closes mobile sidebar, drawer flows, edit and delete flows
