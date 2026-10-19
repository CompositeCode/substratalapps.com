# Substratal Hub for Linux Desktop — Platform Spec

**Standard:** GNOME Human Interface Guidelines with GTK 4 + libadwaita (1.8; minimum 1.5). GNOME is the default desktop on Ubuntu, Fedora, Debian and RHEL; libadwaita apps also run correctly on KDE Plasma and other desktops. A KDE-native variant is outlined in §8.
**Read with:** `SCREENS.md` (all 75 screens with GTK/libadwaita controls), `shared/`.

## 1. Technology

| Concern | Choice |
|---|---|
| Toolkit | GTK 4 + libadwaita; UI files in Blueprint (`.blp`) |
| Language | Rust (gtk4-rs + relm4) — or Python (PyGObject) if the team prefers; choose one |
| API client | openapi-generator (Rust `reqwest` / Python `httpx`) from `openapi.yaml` |
| Secure storage | Secret Service via libsecret (`oo7` in Rust); inside Flatpak via the Secret portal |
| Cache | SQLite in `$XDG_CACHE_HOME/com.substratalapps.Hub`, key in Secret Service |
| Distribution | **Flatpak on Flathub** (GNOME runtime), app ID `com.substratalapps.Hub`, AppStream metainfo with screenshots; optional `.deb`/`.rpm` |

## 2. Window & navigation

- `AdwApplicationWindow`, default 1280×800, minimum 360×294 (adapts to Linux phones).
- `AdwNavigationSplitView`: **sidebar** (`AdwToolbarView` + `AdwHeaderBar`: search button → command palette, title "Substratal", primary menu ☰) with grouped `GtkListBox.navigation-sidebar` (Personal: Home, Inbox (count), Organizations, Account, Plan & Billing, Settings · Administration · Developer incl. Tenant; `AdwSidebar` where available) and **content** (`AdwToolbarView` + `AdwHeaderBar` with `AdwWindowTitle` title/subtitle, actions, window controls).
- Nested `AdwNavigationSplitView` for list-detail sections; `AdwOverlaySplitView` (end side) as the inspector (AUD-01, USR-01).
- Breakpoints (`AdwBreakpoint`): ≤ 600 sp sidebar collapses (sidebar becomes first page); ≤ 900 sp list-detail collapses; header view switcher → bottom `AdwViewSwitcherBar`.
- Primary menu: Preferences (SET-01), Keyboard Shortcuts (`AdwShortcutsDialog`), About (`AdwAboutDialog`), Sign out.
- Sign-in in its own `AdwWindow` before the main window.
- Multiple windows: "Open in New Window" from context menus.

## 3. Interaction

| Interaction | Behavior |
|---|---|
| Context menus | `GtkPopoverMenu` on right-click, long-press (touch), Menu key, Shift+F10; same items as row ⋯ buttons |
| Multi-select | `GtkMultiSelection` in column/list views: click, Shift, Ctrl, Ctrl+A; rubberband selection in `GtkGridView`/`GtkColumnView` (`enable-rubberband`) |
| Drag & drop | `GtkDragSource`/`GtkDropTarget`: users → org in sidebar; reorder pinned tiles; drop CSV/TXT/images/JSON from Files; drag export/audit JSON out as a file (`GdkFileList`) |
| Touch | swipe-back on navigation views (built in); long-press menus; pull-to-refresh not native — use a Refresh button + F5 |
| Hover preview | `GtkPopover` after 500 ms hover on names/IDs; keyboard: Space |
| Undo | `AdwToast` with Undo (10 s) + Ctrl+Z |
| Secret reveal | `AdwDialog can-close=false` until acknowledged |

**Keyboard** (`GtkShortcutController`, listed in the shortcuts dialog):

| Shortcut | Action |
|---|---|
| Ctrl+K / Ctrl+F | Command palette / search |
| Alt+1 … Alt+6 | Home, Inbox, Organizations, Account, Plan & Billing, Settings |
| Ctrl+Shift+U | Mark all as read (Inbox) |
| Ctrl+N | Contextual New |
| Ctrl+R / F5 | Refresh |
| Ctrl+Z | Undo access toggle |
| Delete | Destructive action on selection (confirms) |
| F9 | Toggle sidebar |
| Ctrl+, | Preferences |
| Ctrl+? | Keyboard shortcuts |
| Alt+Left / Escape (pushed page) | Back |
| Ctrl+W / Ctrl+Q | Close window / Quit |

## 4. Platform integration (portals first)

| Concern | Implementation |
|---|---|
| Secrets | libsecret / Secret portal (works with GNOME Keyring and KWallet) |
| Deep links | `x-scheme-handler/substratal` in the `.desktop` file; `DBusActivatable=true` so links reach the running instance |
| App launch / PKCE / billing | `GtkUriLauncher` (OpenURI portal) for apps and Stripe Checkout/Portal; Stripe returns to `https://dashboard.substratalapps.com/billing/return`, whose page offers "Open in the Substratal app" (`substratal://billing/return`); the app polls until `plan = standard` |
| Notifications | Linux desktops have no push service: poll inbox inputs every 60 s while running and raise local `GNotification`s (Notification portal); "Run in background" via the Background portal (opt-in) |
| Badges | Unity Launcher API count (Ubuntu dock) when available; otherwise tray-less — window title prefix |
| Files | `GtkFileDialog` (FileChooser portal) |
| Share | no system share sheet — Copy link / Save file / Send email (`mailto:`) |
| Camera | Camera portal + PipeWire for avatar capture (optional; file picker always available) |
| Autostart | Background portal `RequestBackground(autostart)` (opt-in) |

## 4a. Notifications without push

Linux desktops have no push service the API supports. While the app runs (or in the background, opt-in via the Background portal), poll `unread-count` every 60 s, fetch new items when it rises, and raise local `GNotification`s with buttons per category (routed through GActions such as `app.accept-invitation` and `app.open-route`). NTF-03 explains that notifications arrive while Substratal is running; NTF-04 is replaced by that explanation. Badge: Unity launcher count where available, plus the Inbox sidebar count.

## 4b. Brand & app icon

Assets: `brand-assets/` (see `shared/brand/BRAND.md`). App ID `com.substratalapps.Hub`.

| Item | File | Wiring |
|---|---|---|
| App icon | `icons/hicolor/scalable/apps/com.substratalapps.Hub.svg` and PNGs 16–512 under `icons/hicolor/NxN/apps/` (≤32 px use the small mark) | Install to `/app/share/icons/hicolor/` in the Flatpak manifest |
| Symbolic icon | `icons/hicolor/symbolic/apps/com.substratalapps.Hub-symbolic.svg` (filled outlines only, so GTK can recolour it) | Used by GNOME Shell, notifications and the About dialog's small slots |
| Desktop entry | `com.substratalapps.Hub.desktop` | `Icon=com.substratalapps.Hub`; also registers `x-scheme-handler/substratal` |
| AppStream | `metainfo-branding.snippet.xml` | `<icon>` and `<branding>` colours (light `#E8E8F7`, dark `#15132E`) for GNOME Software / Flathub |
| Logo in UI | `svg/lockup-horizontal*.svg` | Sign-in page (`AdwStatusPage`-style header) and `AdwAboutDialog` uses the app icon. The header bar has no logo; its title is the text "Substratal", per HIG. |


## 4c. Theming

`theme/light/*.css`, `theme/dark/*.css`; wiring and code are in `theme/README.md`.

- Map Settings.theme to `AdwStyleManager:color-scheme`.
- Load the chosen theme's CSS (libadwaita named colours) into one application-priority `CssProvider` and reload it on `notify::dark`.
- Skip the CSS in high contrast. Libadwaita's accent colour preference is overridden by the theme's `accent_bg_color`.
- SET-01: an AdwPreferencesGroup containing a GtkFlowBox of swatch buttons.

## 5. Layout metrics

- libadwaita style classes only: `.boxed-list`, `.card`, `.navigation-sidebar`, `.data-table`, `.pill`, `.suggested-action`, `.destructive-action`, `.flat`, `.linked`; `.title-2`, `.heading`, `.body`, `.caption`, `.dim-label`, `.monospace`.
- `AdwClamp` max 760 for forms/settings; lists full width; header bars ≤ 3 buttons per side.
- System font and accent (user's accent color is honored); symbolic icons per INFORMATION-ARCHITECTURE §5.

## 6. Accessibility

Orca via AT-SPI: accessible roles/labels on badges ("Status: Active") and switches (row title); accessible actions mirror context menus; High Contrast and Large Text supported (sp units only); `gtk-enable-animations` respected; full keyboard operation in every view.

## 7. Testing checklist

- [ ] Polling notifications with GNotification actions inside the Flatpak sandbox; Background portal opt-in.
- [ ] Stripe return via the web page's "Open in the app" link.
- [ ] Ubuntu (GNOME), Fedora (GNOME), KDE Plasma — Flatpak build.
- [ ] Widths 360 → 2560; touchscreen laptop.
- [ ] 11 flows with Orca and keyboard only.
- [ ] Portals: file chooser, OpenURI, notifications, secrets, background — inside the sandbox.

## 8. KDE Plasma variant (optional)

If KDE is a first-class target, build a Qt 6 + **Kirigami** front end on the same core library: `Kirigami.ApplicationWindow` with `GlobalDrawer` (sidebar tree) and `pageStack` columns (list-detail), `FormCard` for settings, `InlineMessage` for banners, `PromptDialog` for confirms, Breeze icons. `SCREENS.md` maps one-to-one; substitute Kirigami controls for the libadwaita column.
