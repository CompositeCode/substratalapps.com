# Components

Semantic components used by `SCREEN-SPECS.md` (the `kind` column), each with a behavior contract and the native control on every platform. Build each once per platform as a reusable view; screens compose only these.

## 1. Behavior contracts (identical everywhere)

| Component | Contract |
|---|---|
| **StatusBadge** | Icon + text label (never color only). Label/icon from `status-semantics.json`. Read by screen readers as "Status: {label}". Non-interactive. |
| **AccessSwitch** | Reflects the *personal* entitlement path. ON→OFF opens ACX-03 confirm (title names person + app; optional reason). OFF→ON applies immediately. Optimistic; on error revert + error toast. After OFF success: toast with **Undo** for 10 s (PATCH back to `active`). Disabled with caption when the caller lacks rights or the row's only path is org-sourced ("Via {org}" link instead). |
| **PathList** | Under a resolved status, lists every access path: source, status, org, member_decision, dates, disabled_reason. Collapsed by default when there is one path. |
| **SecretReveal (KEY-03)** | Modal that cannot be dismissed (Escape, outside click, swipe, back) until "I've copied…" is checked. Monospaced selectable text, Copy button (sensitive clipboard, auto-clear 60 s). Secret never re-rendered after close. Screenshot/recording protection where available. |
| **DestructiveConfirm** | Platform alert/dialog. Title is a question naming the object. Body states consequence. Destructive button uses destructive style and is **not** the default button. Optional reason field when the API accepts one. |
| **EntityList / EntityTable** | Search (debounced 300 ms, server `q` when > 1 page), status filter chips, cursor pagination, virtualized, pull-to-refresh (touch) / ⌘R·Ctrl+R·F5 (keyboard). Preserves scroll, selection and filters on back-navigation. Empty / no-results / forbidden / error states. |
| **MultiSelect + BulkBar** | Desktop/web: click, Shift-click (range), ⌘/Ctrl-click (toggle), ⌘/Ctrl+A, marquee on empty space in grids. Touch: "Select" button or long-press enters selection mode; checkboxes appear. A bulk bar shows count + allowed actions; actions run per row with progress and a partial-failure summary. |
| **ContextMenu** | Right-click / Shift+F10 / Menu key (desktop), long-press (touch), ⋯ button (always visible on hover/focus, and on touch rows). Same items as the row's visible actions. |
| **SchemaForm** | Renders an Application `settings_schema`: string+enum → select; string → text; string `format: date-time` → date-time picker; boolean → switch; integer/number → stepper or number field honoring min/max; object/array → read-only JSON with "Edit on desktop". Each row shows SourceCaption and Reset. |
| **SourceCaption** | "Changed for this app · Reset" / "Using your global setting" / "App default". |
| **IDChip** | Monospaced ID + copy button; copying announces "Copied". |
| **Toast** | Single line + optional action, 5 s (10 s with Undo), pauses on hover/focus, announced politely. Max one visible; queue the rest. |
| **Banner** | Persistent page-level notice (verify email, restricted plan, offline, maintenance, invitation). Dismissible only when informational. |
| **UsageMeter** | Label, `current / limit`, bar; caution at ≥ 80 %, critical at 100 % (semantic roles, not colors). |
| **CommandPalette (GLB-01)** | Opens with ⌘K / Ctrl+K or search button; fuzzy match across navigation, recent items, actions; server lookups for users/orgs (debounced); ID prefixes resolve directly; Enter runs, ⌘/Ctrl+Enter opens in new window/tab where supported. |
| **OTPField** | 6 cells or one field; paste distributes; OS one-time-code autofill; auto-submit on completion. |
| **ThemePicker** | Appearance segmented control (Light · Dark · Match system) plus two radio groups of five theme swatches (light, dark). Each swatch is a miniature of background, sidebar, raised card, accent bar and text lines, with the theme name and a "Default" tag on Moss Paper / Forest Floor. Selecting applies instantly to the whole app with no confirmation; keyboard: arrows move, Space selects; screen readers announce "{name} light theme, 3 of 5, selected". Mobile: 3 columns; desktop: 5. See THEMES.md. |
| **BrandLockup** | The Substratal logo from `brand/` only: horizontal lockup on AUTH-01/02 and About; inline lockup in the web sidebar header; small mark at 24 px and below. Accessible name "Substratal Apps" (decorative when adjacent text already says Substratal). Light/on-dark variant follows the surface. Developer builds: long-press reveals the test-mode toggle. |
| **NotificationRow** | App icon (Substratal mark for `source: platform`), title, 2-line body, relative time, unread dot, "Important" marker for `priority: high`. Opening marks read (optimistic). Swipe/hover actions: Read ⇄ Unread, Archive (Undo). Screen readers read "Unread, Important, {app}: {title}, {time}". |
| **UnreadBadge** | Count from `unread-count` only (never computed from loaded pages). Caps display at 99+. Appears on the Inbox nav item, the bell, the app icon/Dock/taskbar/PWA badge. Announced when it changes while focused. |
| **PushPrimer (NTF-04)** | Explains value before the OS prompt; "Turn on" triggers the OS prompt; "Not now" snoozes 14 days. Never shown before sign-in or more than once per snooze window. |
| **DeviceRow** | App icon, device name (editable), provider, status (On / Off with reason), last registered; actions: Send test (Hub devices), Rename, Remove (confirm). Never shows a token — only `token_hint`. |
| **PlanCard** | Plan name (Free / Standard), what it includes, coverage sources, subscription status badge and period end, primary action (Upgrade / Manage billing / Subscribe the organization). Price from remote config or `unit_amount`. |
| **CheckoutHandoff** | Button with inline progress while creating the session → opens Stripe externally → on return shows "Finishing up…" and polls the subscription (2 s, max 30 s). Prevents double submits with the Idempotency-Key. |
| **CredentialFileField** | Picks/drops/pastes a key file (`.p8`, FCM JSON), validates shape locally, shows only the filename + "Will be stored encrypted", sends once, discards from memory. |
| **ImagePicker** | Camera, photo library, file picker, drag-drop, clipboard paste; crop to square; shows upload progress. (Hidden until OPEN-QUESTIONS #1 resolved.) |

## 2. Native mapping

| Semantic | macOS (SwiftUI) | Windows (WinUI 3) | iOS (SwiftUI) | Android (Compose M3) | Linux (GTK4 + libadwaita) | Web |
|---|---|---|---|---|---|---|
| StatusBadge | `Label` in capsule | Border + FontIcon + TextBlock | `Label` in capsule | non-interactive chip / `Surface` pill | `GtkBox.status-badge` | `<span role="status">`-free pill (icon `aria-hidden`, text visible) |
| AccessSwitch | `Toggle(.switch)` | `ToggleSwitch` | `Toggle` | `Switch` (row `toggleable`) | `GtkSwitch` / `AdwSwitchRow` | `<button role="switch" aria-checked>` |
| PathList | `DisclosureGroup` | `SettingsExpander` | `DisclosureGroup` | expandable `ListItem` | `AdwExpanderRow` | `<details>` / disclosure button |
| EntityList | `List(selection:)` | `ListView` | `List` + `.searchable` | `LazyColumn` + `SearchBar` | `GtkListView` / `.boxed-list` | virtualized `role="list"` / `listbox` |
| EntityTable | `Table` | `ListView` + header grid / CommunityToolkit DataTable | `List` (iPad: `Table`) | `LazyColumn` (expanded: table layout) | `GtkColumnView .data-table` | virtualized `role="grid"` with resizable, sortable columns |
| Tabs (detail) | segmented `Picker` / `TabView` | `SelectorBar` | segmented `Picker` | `PrimaryTabRow` / `SegmentedButton` | `AdwViewSwitcher` / inline switcher | `role="tablist"` |
| Form (modal) | `.sheet` + `Form` | `ContentDialog` / page | `.sheet` + `Form` | full-screen `Dialog` / `ModalBottomSheet` | `AdwDialog` + `AdwPreferencesGroup` | `<dialog>` (desktop) / bottom sheet (phone) |
| DestructiveConfirm | `.confirmationDialog` / `.alert` | `ContentDialog` | `.alert` / `.confirmationDialog` | `AlertDialog` | `AdwAlertDialog` | `<dialog role="alertdialog">` |
| SecretReveal | sheet, `interactiveDismissDisabled` | `ContentDialog` (primary disabled) | sheet, `interactiveDismissDisabled` | `Dialog` (`dismissOnBackPress=false`) + `FLAG_SECURE` | `AdwDialog` `can-close=false` | modal `<dialog>` with `cancel` event prevented |
| ContextMenu | `.contextMenu` | `MenuFlyout` / `CommandBarFlyout` | `.contextMenu` (with preview) | `DropdownMenu` on long-press | `GtkPopoverMenu` | custom menu (`role="menu"`) on `contextmenu` / long-press |
| Toast | bottom HUD overlay (custom) | `InfoBar` / toast region (custom) | capsule overlay (custom) | `Snackbar` | `AdwToast` | `role="status"` live region toast |
| Banner | inline `GroupBox` | `InfoBar` | inline section | banner card | `AdwBanner` | `role="region"` banner |
| UsageMeter | `Gauge(.linearCapacity)` | `ProgressBar` | `Gauge` | `LinearProgressIndicator` | `GtkLevelBar` | `<meter>` |
| SchemaForm rows | `Form` rows | `SettingsCard`s | `Form` rows | `ListItem`s | `Adw*Row`s | form rows (`<select>`, `<input>`, switch) |
| CommandPalette | sheet/panel | title-bar search + `AutoSuggestBox` | search tab / `.searchable` | `SearchBar` expanded | `GtkSearchEntry` popover | `<dialog>` combobox (`role="combobox"` + `listbox`) |
| OTPField | `TextField` `.oneTimeCode` | `TextBox` Number | `TextField` `.oneTimeCode` | `OutlinedTextField` + `ContentType.SmsOtpCode` | `AdwEntryRow` digits | `<input autocomplete="one-time-code" inputmode="numeric">` |
| ImagePicker | `PhotosPicker` + `fileImporter` + paste | `FileOpenPicker` + camera + paste | `PhotosPicker` + camera | Photo Picker + camera intent | `GtkFileDialog` (portal) | `<input type=file accept=image/* capture>` + paste + drop |
| NotificationRow | `List` row with swipe/`contextMenu` | `ListView` item + `SwipeControl` | `List` row + `.swipeActions` | `ListItem` + `SwipeToDismissBox` | `AdwActionRow` + popover menu | `role="listitem"` row with menu + swipe |
| UnreadBadge | `.badge(n)`, `NSApp.dockTile.badgeLabel` | `InfoBadge`, `BadgeUpdateManager` | `.badge(n)`, `UNUserNotificationCenter.setBadgeCount` | `BadgedBox`, launcher badge via notification count | sidebar label count, Unity launcher count | badge element + `navigator.setAppBadge` |
| PushPrimer | sheet → `UNUserNotificationCenter.requestAuthorization` | n/a (local notifications need no prompt) | sheet → `requestAuthorization` | dialog → `POST_NOTIFICATIONS` permission | n/a | dialog → `Notification.requestPermission()` + `pushManager.subscribe` |
| PlanCard | `GroupBox` | Border card | `Section` | `Card` | `.card` | `<section class=card>` |
| CredentialFileField | `fileImporter` + drop + paste | `FileOpenPicker` + drop | `fileImporter` | `OpenDocument` | `GtkFileDialog` + drop | `<input type=file>` + drop + paste |
| MultiSelect | `Table` selection | `ListView SelectionMode=Extended` | `EditButton` / selection | selection mode | `GtkMultiSelection` | `aria-multiselectable` grid |
