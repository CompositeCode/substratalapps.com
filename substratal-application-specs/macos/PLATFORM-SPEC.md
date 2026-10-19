# Substratal Hub for macOS — Platform Spec

**Standard:** Apple Human Interface Guidelines for macOS (macOS 26 Tahoe; Liquid Glass applies automatically to standard controls — don't hand-draw it).
**Read with:** `SCREENS.md` (all 75 screens with macOS controls), `shared/` (product spec, client architecture, components, copy, accessibility, flows).
**Role in the family:** the macOS app is the **reference design** — the Web app's desktop layout is derived from it.

## 1. Technology

| Concern | Choice |
|---|---|
| UI | SwiftUI app lifecycle; AppKit bridges only for `NSTokenField`, `NSWindow.sharingType`, Services |
| Minimum OS | macOS 15 (Sequoia); test on macOS 26 |
| Architecture | Swift Package `SubstratalKit` shared with iOS: generated API client (`swift-openapi-generator`), domain services, Capabilities, persistence. App target holds views + view models (`@Observable`). |
| Concurrency | Swift structured concurrency; one `actor SessionStore` serializes token refresh (single-flight) |
| Persistence | Keychain (refresh token); SwiftData or file cache (encrypted with Data Protection) for catalog/entitlements/profile |
| Networking | URLSession; HTTP/3 enabled; `URLCache` disabled for API (cache is explicit) |
| Distribution | Mac App Store **and** notarized Developer ID build (direct download) with Sparkle updates; same codebase, entitlements differ |

## 2. Scenes & windows

| Scene | Purpose |
|---|---|
| `WindowGroup` main | `NavigationSplitView` sidebar · content · detail. Default 1280×800, min 900×600. State restoration per window. Multiple windows allowed (⌘N) — each with its own navigation. |
| `WindowGroup(for: Route.self)` | Open any user/app/org/role in its own window (⌘-double-click, "Open in New Window"). |
| `Window("Sign in")` | AUTH-* when signed out; 720×560 fixed, hides main windows. |
| `Settings` scene (⌘,) | SET-01 global settings + app preferences (analytics, shortcuts, launch at login). Per-app settings (SET-02) live in the main window. |
| `MenuBarExtra` (opt-in) | Quick launcher: pinned + recent apps, inbox count. |

## 3. Navigation

- **Sidebar** (`List(selection:)`, `.listStyle(.sidebar)`): Personal (Home, Inbox, Organizations, Account, Plan & Billing, Settings) · Administration (Users, Access, Roles, Audit Log, Send notification) · Developer (Applications, API Keys, Webhooks, Tenant), collapsible section headers, `.badge()` counts (unread on Inbox, pending invitations on Organizations). Sidebar search field opens the command palette (GLB-01).
- **Columns:** 2-column for Home, Account, Plan & Billing, Tenant; 3-column for Inbox (list · notification), Settings (scopes · form), Users, Access, Organizations, Roles, Applications, Keys, Webhooks; Audit and a user's Notifications tab use a `Table` with a trailing `.inspector`.
- **Toolbar:** `.navigationTitle` + `.navigationSubtitle`; primary action in `.primaryAction`; grouped items share one capsule (`ToolbarItemGroup`); search via `.searchable(placement: .toolbar)` on list screens; Inbox bell + account menu at trailing edge.
- **Tabs inside detail:** segmented `Picker` in the toolbar's principal position (Overview · Access · Roles · Sessions · Activity).
- **Routing:** a `Route` enum mirrors the routes in SCREEN-SPECS; Universal Links + `substratal://` resolve to `Route`.

## 4. Interaction

| Interaction | Behavior |
|---|---|
| Context menus | `.contextMenu` on tiles, rows, IDs, sidebar items — same items as the row's ⋯ button / toolbar |
| Multi-select | `Table`/`List` selection: click, ⇧-click, ⌘-click, ⌘A; bulk actions in toolbar + context menu (OPEN-QUESTIONS #6) |
| Drag & drop | users → organization in sidebar (invite); app tiles reorder pins; drop CSV/TXT of emails on Users/Members (bulk invite review); drop image on avatar/icon; drop JSON on schema editor; drag export / audit event out as `.json` (`Transferable` with `FileRepresentation`) |
| Quick Look–style preview | `Space` on a selected row opens a preview popover; hovering a name for 0.5 s shows the same as a popover |
| Undo | access toggles register with `UndoManager` (⌘Z) for 10 s |
| Services / Share | Share menu (`ShareLink`) for export files and app links |
| Secret reveal | sheet with `NSWindow.sharingType = .none` while visible (excluded from screen capture) |

**Keyboard shortcuts** (all appear in menus — HIG requires discoverability):

| Shortcut | Action | Menu |
|---|---|---|
| ⌘K | Command palette | Go |
| ⌘1 … ⌘6 | Home, Inbox, Organizations, Account, Plan & Billing, Settings | Go |
| ⇧⌘U | Mark all as read (Inbox) | Edit |
| ⌥⌘1 … ⌥⌘4 | Users, Access, Roles, Audit Log | Go |
| ⌃⌘1 … ⌃⌘4 | Applications, API Keys, Webhooks, Tenant | Go |
| ⌘N / ⇧⌘N | New window / contextual New (Invite user, New role, Create key…) | File |
| ⌘F | Focus search | Edit |
| ⌘R | Refresh | View |
| ⌃⌘S | Toggle sidebar | View |
| ⌥⌘I | Toggle inspector | View |
| ⌘Z | Undo access toggle | Edit |
| ⌘C | Copy ID of selection | Edit |
| ⌘⌫ | Destructive action on selection (confirms) | Edit |
| ⌘↩ | Submit sheet | — |
| Space | Preview | — |

Standard menus: App, File, Edit, View, Go, Window, Help (Help → "API Specification" opens the spec site).

## 5. Platform integration

| Concern | Implementation |
|---|---|
| Secure storage | Keychain `kSecClassGenericPassword`, `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, not synchronizable |
| Re-open lock (opt-in) | Touch ID via `LAContext`; Keychain item with `SecAccessControl(.userPresence)` |
| Password autofill | Associated Domains `webcredentials:dashboard.substratalapps.com`; text content types on AUTH fields; passkeys via `ASAuthorizationController` when flag `passkeys` is on |
| Deep links | Universal Links (`applinks:dashboard.substratalapps.com`) + `substratal://`; `onOpenURL` routes to `Route` |
| App launch (APP-03) | `NSWorkspace.shared.open(url)`; PKCE via `ASWebAuthenticationSession` |
| Billing (BIL-03) | Checkout/Portal open in the default browser; return via Universal Link `/billing/return` (or `/orgs/{id}/subscription`), then poll until `plan = standard`. The Mac App Store build gates Upgrade behind `billing_upgrade_ios` (same App Store policy as iOS); the Developer ID build always shows it. |
| Notifications | APNs as `app_hub` — see §5a; Dock badge = `unread-count` (`NSApp.dockTile.badgeLabel`) |
| Files | `fileExporter`/`fileImporter`; no temp copies |
| Login item | `SMAppService.mainApp` for "Open at login" (Settings) |
| Background | refresh `unread-count` every 60 s while a window is visible; pushes cover the rest |

## 5a. Push notifications (APNs)

| Step | Implementation |
|---|---|
| Bundle id | Same as the iOS app (Universal Purchase) so the single `app_hub` APNs credential (`topic`) reaches both |
| Capability | Push Notifications entitlement (`aps-environment`: development for debug, production for release); Time Sensitive Notifications for `security.*` |
| Permission | NTF-04 primer → `UNUserNotificationCenter.requestAuthorization([.alert, .badge, .sound])` — ask when the person turns notifications on or at the first invitation, not at launch |
| Register | `NSApplication.shared.registerForRemoteNotifications()` → hex token → `POST /users/me/devices {application_id: "app_hub", provider: "apns", token, environment: sandbox/production, name: Host.current().localizedName, app_version}` |
| Refresh / rotate | Re-register once a day on launch; a changed token → `PATCH /users/me/devices/{id}` |
| Categories | `UNNotificationCategory` per COPY-AND-STATES (e.g. `security.new_sign_in`: It was me / Review sessions; `account.organization_invitation`: Accept / View) |
| Delivery | `userNotificationCenter(_:willPresent:)` → in-app banner + inbox refresh; `didReceive` → mark read + route by `action_url` |
| Turn off | NTF-03 "Turn off on this Mac" → `DELETE /users/me/devices/{id}` and `unregisterForRemoteNotifications()` |

## 5b. Brand & app icon

Assets: `brand-assets/` (see `shared/brand/BRAND.md`).

| Item | File | Wiring |
|---|---|---|
| App icon | `Assets.xcassets/AppIcon.appiconset` (16–512 pt @1x/@2x, macOS rounded-rect body baked in; 16/32 px use the small mark) and `AppIcon.icns` | Set *App Icon* to `AppIcon`. Use the `.icns` for non-Xcode packaging. |
| Liquid Glass icon (macOS 26+) | `icon-composer-layers/` | Build `AppIcon.icon` in Icon Composer. The set above is the fallback. |
| Logo images and colours | `Assets.xcassets/Brand/*`, identical to iOS (lives in `SubstratalKit` resources) | `Image("Brand/BrandLockup")` |
| Menu-bar extra (optional) | `Assets.xcassets/MenuBarIcon` (template SVG) and `menubar-template/*.png` | `MenuBarExtra { … } label: { Image("MenuBarIcon") }`. It's a template image, so the system tints it. |
| Dock badge | — | `NSApp.dockTile.badgeLabel` from `unread-count` |

Placement: the sign-in window shows the horizontal lockup. **About Substratal** (standard About panel) uses the app icon. Sidebars and toolbars have no logo, per HIG.


## 5c. Theming

Same `theme/SubstratalTheme.swift` as iOS (in `SubstratalKit`).

- The sidebar uses `palette.sidebar`; keep the translucent sidebar material and set its tint to `palette.sidebar` at 92 %.
- Content panes use `palette.raised` and the window background uses `palette.background`.
- List and table selection uses `palette.accent` with `palette.onAccent` text, not the system accent.
- Set `NSApp.appearance` from Settings.theme (`nil` for Match system).
- SET-01 shows the swatches as a five-column grid in the Settings window.

## 6. Layout metrics

- Sidebar 220 pt default (resizable 180–320). Content column 280–360 pt. Inspector 280–340 pt.
- Content insets 20 pt; 8 pt between related controls; 20 pt between groups.
- Table rows 24 pt (text-only) / 32 pt (avatars). Form rows ≥ 28 pt.
- Text styles only (`.title2` detail titles, `.headline`, `.body` 13 pt, `.callout`, `.caption`); IDs `.monospaced()`.
- Icons: SF Symbols per INFORMATION-ARCHITECTURE §5.

## 7. Accessibility

VoiceOver rotor on tables; tiles and rows expose custom actions (`.accessibilityActions`) for everything in their context menu; switches labeled "Access to {app}". Full Keyboard Access; visible focus; Dynamic Type n/a on macOS but respect system text size where SwiftUI scales; Increase Contrast, Reduce Motion, Reduce Transparency honored by system controls.

## 8. Testing checklist

- [ ] APNs: permission, register (sandbox + production), daily refresh, token change, sign-out deactivation, category actions, Dock badge from `unread-count`.
- [ ] Stripe Checkout/Portal round trip in test mode (Developer ID build) and the "Finishing up…" poll.
- [ ] All 75 screens via routes; flags on/off per phase.
- [ ] Persona matrix (six personas) — sidebar contents and every action button.
- [ ] 11 flows with mouse only, keyboard only (Full Keyboard Access), VoiceOver.
- [ ] Multi-window: same user open in two windows stays consistent after edits (refetch on focus).
- [ ] Sleep/wake and network change: token refresh single-flight, no duplicate writes.
- [ ] App Store sandbox build and Developer ID build both pass.
