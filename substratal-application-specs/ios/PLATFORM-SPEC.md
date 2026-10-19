# Substratal Hub for iOS & iPadOS — Platform Spec

**Standard:** Apple Human Interface Guidelines (iOS/iPadOS 26, Liquid Glass via standard components).
**Read with:** `SCREENS.md` (all 75 screens with SwiftUI controls), `shared/`.

## 1. Technology

| Concern | Choice |
|---|---|
| UI | SwiftUI, minimum iOS 17 |
| Shared code | `SubstratalKit` Swift package with macOS: generated API client, services, Capabilities, persistence |
| Secure storage | Keychain `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, never iCloud-synced |
| Cache | SwiftData / files with `NSFileProtectionComplete` |
| Distribution | App Store (universal iPhone + iPad); TestFlight for betas |

## 2. Navigation

**iPhone** — `TabView`, each tab its own `NavigationStack` (path persisted with `@SceneStorage`):

| Tab | Root | Pushes |
|---|---|---|
| Home | APP-01 | APP-02 (sheet), APP-04, SET-02, APP-05 |
| Inbox (badge) | NTF-01 | NTF-02 |
| Orgs | ORG-01 | ORG-03/06/08, BIL-02, ORG-04 (sheet) |
| Manage | GLB-02 hub (Administration + Developer) | USR-*, ACX-*, ROL-*, AUD-*, NTF-05/06, APL-* (incl. APL-07), KEY-*, WHK-*, BIL-04 |
| Account | ACC-01 | ACC-04, NTF-03, ACC-07, BIL-01 (Plan & Billing), SET-01 (Settings) |

The Manage tab is **removed** (not disabled) when the person has no admin or developer rights — an end user sees 4 tabs: Home · Inbox · Orgs · Account.

**iPad** — `TabView` with `.tabViewStyle(.sidebarAdaptable)`: sidebar shows the full desktop tree (same as macOS); sections with list-detail use `NavigationSplitView`. Supports Split View, Slide Over, Stage Manager, multiple windows (scene per window, `WindowGroup(for: Route.self)`), and resizing without losing state.

## 3. Interaction

| Interaction | Behavior |
|---|---|
| Long-press | `.contextMenu` with preview on tiles/rows/IDs (same items as ⋯ / toolbar) |
| Swipe actions | `.swipeActions` on rows: leading safe (Accept, Turn on, Mark read), trailing destructive (Turn off, Remove, Revoke → confirm). Full swipe only for non-destructive. |
| Pull to refresh | `.refreshable` on every list |
| Multi-select | `EditButton` / "Select" → `List(selection:)`; two-finger pan to select on iPhone/iPad; bulk actions in bottom toolbar |
| Drag & drop | iPad: drag users onto an org in the sidebar; drag app tiles to reorder pins; drop CSV/TXT/images/JSON from Files/other apps (`.dropDestination`); drag export/audit event out (`Transferable`) |
| Pointer (iPad trackpad/mouse) | `.hoverEffect`, right-click = context menu, hover preview popovers on names |
| Keyboard (iPad) | same shortcuts as macOS where valid (⌘K, ⌘F, ⌘R, ⌘N, ⌘Z, ⌘⌫, ⌘↩, ⌘1–4) via `.keyboardShortcut`, visible in the ⌘-hold overlay |
| Pencil | precise pointer; Scribble in all text fields (default); hover with Pencil Pro shows previews |
| Haptics | `.sensoryFeedback(.success)` on grant/turn-on, `.warning` before turn-off confirm, `.selection` on segmented changes |
| Toast | glass capsule overlay above the tab bar (custom), VoiceOver announcement |
| Undo | toast Undo (10 s) + shake-to-undo |
| Secret reveal | sheet with `interactiveDismissDisabled`; content hidden in app switcher snapshot and replaced during screen recording (`UIScreen.isCaptured`) |

## 4. Platform integration

| Concern | Implementation |
|---|---|
| Autofill | Associated Domains (`webcredentials:`), text content types; Passwords app saves on sign-in |
| Passkeys | `ASAuthorizationController` platform passkeys when flag `passkeys` is on |
| One-time codes | `.textContentType(.oneTimeCode)` (Passwords app supplies TOTP codes when the user stored the setup there via `otpauth://`) |
| Biometric lock | optional Face ID/Touch ID on open (`NSFaceIDUsageDescription`) |
| Deep links | Universal Links + `substratal://` → `Route` |
| App launch / PKCE | `openURL` / `ASWebAuthenticationSession` |
| Billing | BIL-01/02 show plan, coverage and status; Checkout/Portal open in Safari (`openURL`) with Universal Link return and polling. **Upgrade** is gated by `billing_upgrade_ios`, default off until App Store policy review (OPEN-QUESTIONS #10): Standard is a consumer subscription unlocking digital apps. |
| Push | APNs as `app_hub` — see §4a; app icon badge = `unread-count` via `UNUserNotificationCenter.setBadgeCount` |
| Share | `ShareLink` for links and export files |
| Files | `fileExporter`/`fileImporter`, document picker; camera + `PhotosPicker` for avatar |
| Widgets / Controls | Home Screen widget "Quick launch" (pinned apps); Control Center control "Open Substratal" |
| App Intents | "Open app {name}", "Turn off access for {person} in {app}" (requires confirmation) for Shortcuts/Siri/Spotlight |
| Privacy | `PrivacyInfo.xcprivacy`; in-app account deletion = ACC-08 (App Store requirement) |

## 4a. Push notifications (APNs)

| Step | Implementation |
|---|---|
| Bundle id | Shared with the macOS app (Universal Purchase) — one `app_hub` APNs credential/topic |
| Capabilities | Push Notifications (`aps-environment`), Time Sensitive Notifications (for `security.*` alerts sent with `interruption-level: time-sensitive`), Background Modes → Remote notifications (silent badge refresh) |
| Permission | NTF-04 primer at the first meaningful moment after sign-in → `requestAuthorization([.alert, .badge, .sound])`; never before sign-in; "Not now" snoozes 14 days |
| Register | `UIApplication.registerForRemoteNotifications()` → token hex → `POST /users/me/devices {application_id: "app_hub", provider: "apns", token, environment (sandbox for debug/dev builds, production for TestFlight/App Store), name: device name, app_version}`; store the `dvc_` id |
| Refresh / rotate | Re-register once a day on launch; a changed token in `didRegisterForRemoteNotificationsWithDeviceToken` → `PATCH /users/me/devices/{id}` |
| Categories & actions | `UNNotificationCategory` per category: `security.new_sign_in` (It was me, Review sessions), other `security.*` (Secure my account), `account.organization_invitation` (Accept — `.authenticationRequired`, View), `account.access_granted` (Open app) |
| Foreground | `willPresent` → in-app banner (no system banner), refresh inbox + badge |
| Tap / action | `didReceive` → `PATCH …/notifications/{id} {state:read}` → route by `action_url`; Accept → `POST /organizations/{id}/members/me/accept` |
| Turn off | NTF-03 → `DELETE /users/me/devices/{id}`; OS-level revocation needs no call (server deactivates on next send) |
| Lock screen | Show title/body as sent; respect "Show Previews" — never add detail |

## 4b. Brand & app icon

Assets: `brand-assets/` (see `shared/brand/BRAND.md` for rules).

| Item | File | Wiring |
|---|---|---|
| App icon | `Assets.xcassets/AppIcon.appiconset` holds single-size 1024 px icons: light (opaque, no alpha), dark (transparent) and tinted (grayscale). | Set *App Icon* to `AppIcon`. The system derives every size. |
| Liquid Glass icon (iOS 26+) | `icon-composer-layers/` (background + 3 groups) | Build `AppIcon.icon` in Icon Composer and add it to the target. The flat set above is the fallback. |
| Older toolchains | `legacy-sizes/Icon-*.png` | Only if the project still needs per-size slots |
| Logo images | `Assets.xcassets/Brand/BrandMark`, `BrandMarkSmall`, `BrandLockup`, `BrandLockupInline`, `BrandLockupStacked` (SVG, preserves vector, light + dark) | `Image("Brand/BrandLockup").accessibilityLabel("Substratal Apps")` |
| Brand colours | `Assets.xcassets/Brand/*.colorset` (light + dark) | `Color("Brand/BrandPink")`. For icons and logo only; UI colour comes from `theme/SubstratalTheme.swift` (§4c). |
| Launch screen | `LaunchScreen.md` | `UILaunchScreen` → `Brand/BrandMark` on `Brand/IconBackground` |

Placement: horizontal lockup centred above the AUTH-01 form, with 32 pt above it. Settings › About shows the lockup and version. Notification content uses the app icon. The iPad sidebar header has no logo.


## 4c. Theming

`theme/SubstratalTheme.swift` (in `SubstratalKit`) and `theme/Assets.xcassets/AccentColor` (defaults).

- Wrap the root view in `.substratalTheme(store)`. Views read colours from `@Environment(\.palette)`, never from literals.
- `List`/`Form` grouped background uses `palette.background`, cells use `palette.raised` (`.listRowBackground`), and tint comes from `palette.accent` (or `accentStrong` under Increase Contrast).
- The tab bar, navigation bars and sheets keep system materials. Only tint and the content backgrounds change.
- SET-01 swatches are a three-column grid inside a Form section.
- Persist the choice per THEMES.md §1. Reference screens are in `screens/light` and `screens/dark`.

## 5. Layout metrics

- Inset-grouped lists; system corner radii and margins (don't hard-code).
- Dynamic Type text styles only: `.largeTitle` tab roots, `.headline`, `.body`, `.subheadline` row subtitles, `.footnote` captions; IDs `.monospaced()`.
- Targets ≥ 44 × 44 pt. Grid: 2 columns (iPhone), 3–5 (iPad, adaptive 160 pt).
- Icons: SF Symbols per INFORMATION-ARCHITECTURE §5.

## 6. Accessibility

Dynamic Type to AX5 (grid → 1 column, trailing row content wraps below via `ViewThatFits`); VoiceOver custom actions mirror context menus and swipe actions; Voice Control labels match visible text; Bold Text, Reduce Motion, Reduce Transparency, Increase Contrast honored; Switch Control reachable for all actions.

## 7. Testing checklist

- [ ] APNs end to end with a Hub device test (NTF-03): register, rotate, sign-out deactivation, time-sensitive security alerts, Accept-invitation action from the lock screen.
- [ ] Billing: plan states (Free, Standard own, Standard via org, past_due, canceled); Upgrade hidden while `billing_upgrade_ios` is off.
- [ ] iPhone SE → Pro Max; iPad mini → Pro 13" in every multitasking size; keyboard + trackpad + Pencil attached simultaneously.
- [ ] 11 flows with VoiceOver, Switch Control, largest Dynamic Type.
- [ ] Persona matrix — tab set and every action.
- [ ] Universal Links from Mail for verify/reset/invite with app installed and not installed.
- [ ] Background/foreground: refresh single-flight, inbox badge correct.
