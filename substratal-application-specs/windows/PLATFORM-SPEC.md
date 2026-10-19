# Substratal Hub for Windows — Platform Spec

**Standard:** Fluent 2 Design System and Windows 11 app design guidance.
**Read with:** `SCREENS.md` (all 75 screens with WinUI controls), `shared/`.

## 1. Technology

| Concern | Choice |
|---|---|
| UI | WinUI 3 (Windows App SDK 1.7+), C# / .NET 8+ |
| Pattern | MVVM with CommunityToolkit.Mvvm; DI via Microsoft.Extensions.Hosting |
| Controls | WinUI + CommunityToolkit (SettingsCard/Expander, Segmented, TokenizingTextBox, DataTable/TableView, GridSplitter) |
| API client | Kiota (or NSwag) from `openapi.yaml`; `HttpClient` with `SocketsHttpHandler`, HTTP/3 enabled |
| Secure storage | Credential Locker (`PasswordVault`) or DPAPI-protected file (`ProtectedData`, CurrentUser) |
| Cache | SQLite (Microsoft.Data.Sqlite) in `LocalCacheFolder`, encrypted with DPAPI-protected key |
| Minimum | Windows 10 22H2 (WinAppSDK); design target Windows 11 |
| Distribution | MSIX via Microsoft Store + App Installer (sideload, auto-update); single-instance (`AppInstance.FindOrRegisterForKey`) |

## 2. Window & navigation

- **Title bar:** custom (`ExtendsContentIntoTitleBar`), tall (48 epx): app icon + "Substratal"; centered search `AutoSuggestBox` (Ctrl+K / Ctrl+E) that is the command palette; caption buttons. Backdrop Mica.
- **NavigationView** `PaneDisplayMode=Left`: group headers Personal (Home, Inbox, Organizations), Administration (Users, Access, Roles, Audit Log, Send notification), Developer (Applications, API Keys, Webhooks, Tenant); **FooterMenuItems: Account, Plan & Billing, Settings** (Fluent convention). `InfoBadge` with the unread count on Inbox and pending invitations on Organizations.
- Adaptive: Left ≥ 1008 epx; LeftCompact 641–1007; LeftMinimal ≤ 640. List-detail pages collapse to single pane < 900 epx with drill-in + back.
- **Pages:** `Frame` navigation, page header = TitleLarge + `BreadcrumbBar` for drill-ins (Access › Dana Okafor) + right-aligned `CommandBar` (primary action uses AccentButtonStyle).
- **Sign-in** is a full-window page before the shell is created (no NavigationView).
- Multiple windows: "Open in new window" from context menus creates a secondary `Window` with a single page.

## 3. Interaction

| Interaction | Behavior |
|---|---|
| Context menus | `ContextFlyout` (`MenuFlyout` / `CommandBarFlyout`) on tiles, rows, IDs; opens on right-click, Shift+F10, Menu key, and **press-and-hold** on touch |
| Multi-select | `ListView SelectionMode=Extended` (click, Shift, Ctrl, Ctrl+A); touch: "Select" toggles checkbox mode; bulk actions in CommandBar |
| Drag & drop | users → org in NavigationView (invite); reorder pinned tiles; drop CSV/TXT/images/JSON from Explorer (`AllowDrop`, `DataPackageView.GetStorageItemsAsync`); drag export/audit event out as `StorageFile` |
| Hover preview | `ToolTip` with rich content (person/app card) after 500 ms on names/IDs; keyboard: Space |
| Touch | press-and-hold menus, swipe on list items via `SwipeControl` (leading Accept/Turn on, trailing Turn off/Remove → confirm), pull-to-refresh via `RefreshContainer` |
| Pen | precise pointer; inking not used |
| Undo | access toggle InfoBar with Undo (10 s) + Ctrl+Z |
| Secret reveal | ContentDialog; window `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` while visible |

**Keyboard** (declared with `KeyboardAccelerator`, shown in tooltips; Alt access keys on CommandBar):

| Shortcut | Action |
|---|---|
| Ctrl+K / Ctrl+E | Command palette / search |
| Ctrl+1 … Ctrl+6 | Home, Inbox, Organizations, Account, Plan & Billing, Settings |
| Ctrl+Shift+U | Mark all as read (Inbox) |
| Ctrl+Shift+1 … 4 | Users, Access, Roles, Audit Log |
| Ctrl+Alt+1 … 4 | Applications, API Keys, Webhooks, Tenant |
| Ctrl+N | Contextual New |
| Ctrl+R / F5 | Refresh |
| Ctrl+Z | Undo access toggle |
| Ctrl+C | Copy ID |
| Delete | Destructive action on selection (confirms) |
| Alt+Left / Backspace (not editing) / mouse back | Back |
| F6 | Cycle focus: nav pane → list → detail |
| Shift+F10 / Menu | Context menu |
| Ctrl+Enter | Submit dialog |

## 4. Platform integration

| Concern | Implementation |
|---|---|
| Secure storage | Credential Locker; Windows Hello (`UserConsentVerifier`) to unlock on launch (opt-in) |
| Password / passkeys | Windows Hello passkeys via WebAuthn APIs when flag `passkeys` is on |
| Deep links | protocol `substratal:` in manifest; App URI Handler for `https://dashboard.substratalapps.com`; activation redirected to the running instance |
| App launch / billing | `Launcher.LaunchUriAsync` (default browser) for app launch and Stripe Checkout/Portal; the App URI Handler brings `/billing/return` back to the app, which polls until `plan = standard`. PKCE via browser + protocol return. Microsoft Store builds: confirm Store policy for third-party payment of a digital subscription before enabling Upgrade. |
| Notifications | **No Windows push provider in the API** (APNs/FCM/Web Push only) — see §4a; taskbar badge (`BadgeUpdateManager`) = `unread-count` |
| Share | `DataTransferManager` share sheet for export files and links |
| Files | `FileSavePicker` / `FileOpenPicker` (with window handle interop) |
| Clipboard | `ClipboardContentOptions.IsAllowedInHistory=false`, `IsRoamable=false` for secrets |
| Startup | `StartupTask` for "Open at sign-in" (opt-in) |
| Jump list | Taskbar jump list: Home, Users, API Keys, Audit Log, recent users |

## 4a. Notifications without push (polling + local app notifications)

| Step | Implementation |
|---|---|
| Source | `GET /users/me/notifications/unread-count` every 60 s while running (window open or minimized to tray), and `GET /users/me/notifications?created_after={lastSeen}` when the count rises |
| Show | `AppNotificationManager` local app notifications for each new notification since `lastSeen`, title/body as sent, buttons per category (Accept / Review sessions / Open) |
| Background | Optional "Keep checking for notifications in the background" (SET-01) → `StartupTask` + tray icon; off by default |
| Activation | Notification arguments → mark read → route by `action_url` |
| Badge | `BadgeUpdateManager` numeric badge = `unread_count`; `InfoBadge` on Inbox |
| Devices | NTF-03 lists other devices; "This device" explains Windows notifications come from the app while it's running. NTF-04 is replaced by this explanation |
| Future | If the API adds a `wns` provider (OPEN-QUESTIONS #2), register the WNS channel URI as a Device and drop polling |

## 4b. Brand & app icon

Assets: `brand-assets/` (see `shared/brand/BRAND.md`).

| Item | File | Wiring |
|---|---|---|
| Window / taskbar icon | `Substratal.ico` (16–256; 16–24 px use the small mark). Use `Substratal-dark.ico` on dark title bars. | `AppWindow.SetIcon(...)`. Also set `ApplicationIcon` in the .csproj. |
| MSIX visual assets | `Assets/`: Square44x44Logo (scale-100…400, targetsize-16…256 with `altform-unplated` for dark taskbar and `altform-lightunplated` for light), Square150x150Logo, Wide310x150Logo, SmallTile, LargeTile, StoreLogo, SplashScreen, BadgeLogo | Copy into the packaging project's `Assets/` and merge `Package.appxmanifest.snippet.xml`. |
| Title bar | 16 px small mark + "Substratal" (custom title bar, `ExtendsContentIntoTitleBar`) | Use the matching `svg/mark-small*.svg` with an `ImageIcon`. |
| Sign-in / About | `svg/lockup-inline*.svg` | Render via `SvgImageSource`. Accessible name "Substratal Apps". |
| Toasts | App logo comes from the package. `appLogoOverride` is only used for per-app icons in the inbox. | |

Plated tiles use the tint surface (`#E8E8F7`) so they read on both light and dark Start. Unplated taskbar icons switch palette by theme.


## 4c. Theming

`theme/SubstratalThemes.xaml` (merge first in App.xaml), `theme/Themes/{Light,Dark}/*.xaml` and `theme/ThemeService.cs`.

- Theme dictionaries override `SystemAccentColor*`, `TextFillColor*`, `SolidBackgroundFillColorBase`, `LayerFillColorDefault`, `CardBackgroundFillColorDefault` and `DividerStrokeColorDefault`, so stock WinUI controls pick up the theme. Custom UI uses `Substratal*Brush` via `{ThemeResource}`.
- `ThemeService.Apply` swaps the dictionaries and sets `RequestedTheme`. Tint Mica with `background` (`ThemeService.TintBackdrop`).
- High Contrast: the empty `HighContrast` dictionary keeps system colours.
- SET-01 uses a SettingsExpander containing a GridView of swatches.

## 5. Layout metrics

- Page padding 36 epx (24 compact); SettingsCard min height 68, 4 epx gap; 24 epx between groups.
- Control corner radius 4, overlays 8 (theme resources, not hard-coded).
- Type ramp: TitleLarge 28 (page titles), Subtitle 20 (detail/dialog titles), BodyStrong 14 (group headers), Body 14, Caption 12; IDs in Cascadia Mono.
- Icons: Segoe Fluent Icons / Fluent System Icons per INFORMATION-ARCHITECTURE §5.

## 6. Accessibility

Narrator: `AutomationProperties.Name` on badges/switches ("Access to Timetrack, On"); `LiveSetting=Polite` on InfoBars/toasts; grid patterns on tables. Text scaling to 225 %; High Contrast themes (no hard-coded colors — use theme/system resources). Every SwipeControl/press-and-hold action is also in the context menu and CommandBar.

## 7. Testing checklist

- [ ] Polling notifications: local app notifications for new items, actions, taskbar badge, background option; no duplicate toasts after restart.
- [ ] Stripe Checkout/Portal round trip via default browser and App URI Handler return.
- [ ] All screens via routes and NavigationView; persona matrix.
- [ ] 100 %, 150 %, 200 % scaling; touch laptop (touch + pen + trackpad simultaneously); tablet posture.
- [ ] Keyboard-only and Narrator passes of the 11 flows; Accessibility Insights FastPass clean.
- [ ] Single-instance deep-link activation while running/closed.
- [ ] MSIX install/upgrade via Store and App Installer.
