# Substratal Hub for Web — Platform Spec

A modern, responsive web application and installable PWA that works equally well on phones, tablets and desktop browsers from one codebase. Its **base design is the macOS app** (sidebar · list · detail · inspector, toolbar with title/subtitle, sheets for forms, grouped settings rows), adapted to the web and to each device's size, input and system settings.

**Read with:** `SCREENS.md` (all 75 screens with web controls), `shared/` (product spec, client architecture, components, copy, accessibility, flows), `screens/` (reference renders at phone / tablet / desktop).

---

## 1. Technology

| Concern | Choice | Why |
|---|---|---|
| Language / build | TypeScript (strict), Vite | fast builds, route-level code splitting |
| UI | React 19 | team familiarity, ecosystem |
| Accessible primitives & interactions | **React Aria Components** (Adobe) | press/long-press, hover that ignores touch emulation, keyboard + typeahead, multi-select with Shift/Cmd/Ctrl, drag-and-drop incl. files to/from the OS, resizable table columns, ARIA patterns done right |
| Routing | TanStack Router (typed routes = SCREEN-SPECS routes) | search-param state for filters, preloading on intent |
| Server state | TanStack Query | caching per CLIENT-ARCHITECTURE §5, background refetch on focus, optimistic updates with rollback |
| API client | `openapi-typescript` + `openapi-fetch` generated from `openapi.yaml` | typed requests/responses |
| Virtualization | TanStack Virtual | users / audit / deliveries tables |
| Code editor | CodeMirror 6 (JSON) | APL-04 schema editor, diff view |
| PWA | Workbox (`vite-plugin-pwa`, injectManifest) | precache shell, runtime caching, push, background sync for analytics only |
| Styling | CSS with cascade layers, custom properties, container queries, logical properties | responsive + RTL without JS |
| i18n | `Intl.*` + ICU MessageFormat (FormatJS) | locale, plurals, dates, relative time, lists |
| Testing | Vitest, Playwright (Chromium/WebKit/Firefox, mobile emulation + touch), axe-core | — |

Everything below is implementable with the platform APIs listed; each has a fallback (§11).

## 2. Hosting & session security

- Served from `https://dashboard.substratalapps.com` (the host the API spec uses for billing returns) over HTTP/2+3, HSTS preload. The Web Hub also hosts the Phase 3 hosted sign-in page.
- **Backend-for-frontend (BFF):** a thin same-origin service holds the platform **refresh token in an `HttpOnly; Secure; SameSite=Strict` cookie** and exposes `/bff/session/refresh` and `/bff/session/logout`. The SPA keeps the 15-minute **access token in memory only** and calls the API directly with `Authorization: Bearer`. Never put refresh tokens in `localStorage`/IndexedDB. (If a BFF is ruled out, see OPEN-QUESTIONS #14 for the fallback and its risk.)
- CSP: `default-src 'self'; script-src 'self' 'strict-dynamic' 'nonce-…'; connect-src 'self' https://api.substratalapps.com; frame-ancestors 'none'; require-trusted-types-for 'script'`. No third-party scripts on any route.
- CSRF: BFF endpoints require `SameSite=Strict` cookie + custom header `X-Substratal-CSRF`.
- Logout: BFF responds with `Clear-Site-Data: "cache", "storage"`; the service worker also deletes its runtime caches.
- Tokens from emailed links (`?token=`) are exchanged immediately, then removed from the URL with `history.replaceState`.

## 3. Responsive layout (based on the macOS design)

Breakpoints by **viewport width** for the shell; **container queries** for components (a table in a narrow pane reflows to cards even on desktop).

| Class | Width | Shell | Navigation | Panes | Modals |
|---|---|---|---|---|---|
| **Compact** (phones) | < 768 px | single pane, thumb-friendly | **bottom navigation bar**: Home · Orgs · Admin · Developer · Account (Admin/Developer = hub lists GLB-02, hidden without rights) | one pane; detail pushes with back button; list ↔ detail via View Transitions | **bottom sheets** with grab handle and snap points (half / full); confirms as bottom action sheets |
| **Medium** (tablets, small laptops) | 768 – 1199 px | two panes | **collapsible sidebar** (macOS sidebar tree) — overlay drawer by default, can be pinned; toggle button + `[` key + edge swipe | list + detail side by side (list 320 px); inspector opens as overlay panel | centered dialogs (max 560 px); popovers become side panels |
| **Expanded** (desktop) | ≥ 1200 px | dense multi-pane | **persistent sidebar** (resizable 200–320 px, collapsible) with sections Personal · Administration · Developer, exactly as macOS | sidebar · list · detail · optional **inspector**; every divider **resizable** (pointer drag, keyboard `role=separator` with arrows), sizes remembered per route | centered dialogs; popovers anchored; toolbars with icon+label buttons |
| **Large** | ≥ 1600 px | as expanded | as expanded | inspector shown by default on AUD-01, USR-01, ACX-05 | — |

Shell anatomy (expanded), mirroring macOS:

```
┌ sidebar ────────┬ toolbar: title / subtitle ········ search · [actions] · bell · avatar ┐
│ Search  ⌘K      ├──────────────────────┬───────────────────────────────┬──────────────┤
│ PERSONAL        │ list pane            │ detail pane                   │ inspector    │
│  Home           │ (search, chips,      │ header: title, ID chip, badge │ (optional)   │
│  Organizations ●│  virtualized rows)   │ tabs                          │              │
│  Account        │                      │ content                       │              │
│  Settings       │                      │                               │              │
│ ADMINISTRATION  │                      │                               │              │
│ DEVELOPER       │                      │                               │              │
└─────────────────┴──────────────────────┴───────────────────────────────┴──────────────┘
```

Rules:
- Safe areas: `env(safe-area-inset-*)` on the bottom nav, sheets and toolbar (notches, home indicator); `viewport-fit=cover`.
- Dynamic viewport units (`dvh`) for full-height panes so mobile browser chrome doesn't hide the bottom nav.
- Content width capped at 760 px for forms/settings, tables use full pane width.
- 320 px minimum width with no horizontal scroll (WCAG reflow); browser zoom to 400 % keeps content usable (reflows to compact).
- On-screen keyboard: use `visualViewport` to keep the focused field and sheet buttons visible; `interactive-widget=resizes-content`.
- Orientation and window resizes (split-screen on iPad/Android, desktop window snapping) re-evaluate the class live without losing state; selection and scroll persist when panes collapse.

## 4. Input adaptation

The app adapts to **how** the person is interacting, not only to screen size. Detect with media queries and Pointer Events; never with user-agent sniffing.

| Signal | Detection | Adaptation |
|---|---|---|
| Fine pointer present | `(any-pointer: fine)` | denser rows (32 px), hover affordances |
| Coarse pointer present | `(any-pointer: coarse)` | 44 px minimum targets, 52 px rows, visible ⋯ buttons on rows, swipe affordances |
| Hover capable | `(any-hover: hover)` | hover previews, reveal row actions on hover/focus |
| Current input | `pointerType` of the last `pointerdown` (`mouse`, `pen`, `touch`) + keydown | switch hit-slop and long-press behavior per event; show keyboard focus rings only after keyboard use (`:focus-visible`) |
| Hardware keyboard | first non-typing keydown | show shortcut hints in tooltips, menus and the palette |

**Tablets support touch, pencil, trackpad and keyboard at the same time.** Handle every interaction with Pointer Events per event (`pointerType`) instead of a global "mode": a trackpad pointer gets hover previews and right-click menus, a finger gets long-press and swipe on the same screen, a pencil acts as a precise pointer (marquee selection, pen hover where supported, Scribble into text fields works natively), and a keyboard gets the full shortcut set.

## 5. Interaction model

**Every gesture and every shortcut accelerates an action that also has a visible button or menu item.** Screen-reader and keyboard users reach every action without gestures.

### 5.1 Desktop / pointer

| Interaction | Where | Behavior | Visible equivalent |
|---|---|---|---|
| Right-click context menu | tiles (APP-01), rows in every list/table, sidebar items, IDs | same items as the row's ⋯ menu; `Shift+F10` / Menu key opens it at focus | ⋯ button on hover/focus |
| Hover preview | user/app/org names and IDs anywhere, tiles | after 500 ms hover (or focus + `Space`), a preview card: avatar/icon, status, key facts, primary actions; dismiss on leave/Esc; never on touch | open the item |
| Multi-select | USR-01, ACX-05, KEY-01, APP-01 tiles, ORG-03 members | click selects; `Shift`+click range; `⌘/Ctrl`+click toggle; `⌘/Ctrl+A` all loaded; arrow keys + `Shift` extend | "Select" button → checkboxes |
| Marquee selection | APP-01 grid, tables (from empty space / row gutter), ORG-03 | drag a rectangle from empty space to select intersecting items; with `⌘/Ctrl` adds to selection; auto-scrolls at edges | "Select" button |
| Bulk bar | when ≥ 1 selected | sticky bar: count, allowed actions (Suspend, Resend invitation, Turn off access, Remove from org, Revoke keys), Clear | — |
| Command palette | everywhere | `⌘K` / `Ctrl+K` (GLB-01): navigate, launch apps, find users/orgs by name or ID, run actions | search field in sidebar/toolbar |
| Drag & drop in app | sidebar + lists | drag users (single or selection) onto an organization in the sidebar → ORG-04 prefilled; drag app tiles to reorder pinned apps; drag a role onto a user (ROL-01 → USR-03 Roles tab) → assign (confirm) | menu actions "Add to organization…", "Pin", "Assign role…" |
| Drag files **in** from the desktop | USR-01, ORG-04, ACC-01 avatar, APL-04, APL-02 icon | drop a CSV/TXT of emails → bulk invite review; drop an image → avatar/icon crop; drop a JSON file → schema editor diff | file picker buttons |
| Drag files **out** to the desktop | ACC-07/USR-03 export, AUD-02 event, KEY-03 never | drag an export card / audit event out as a `.json` file (`DownloadURL` drag data in Chromium); other browsers fall back to the Download button | "Download" button |
| Resizable panels | all split panes, table columns | pointer drag on dividers/column edges; double-click resets; keyboard: focus separator + arrows | — |
| Toolbars | every list/detail | icon + label buttons; overflow menu when narrow (container query) | — |

### 5.2 Touch

| Interaction | Where | Behavior | Visible equivalent |
|---|---|---|---|
| Long-press menu | tiles, rows, IDs | 500 ms press → haptic (`navigator.vibrate(10)` where allowed) → context menu sheet with a preview header; release without moving does not activate the item | ⋯ button on every row |
| Swipe actions | list rows (USR-01, ACX-01, ORG-01/03, KEY-01, WHK-03, inbox) | horizontal swipe reveals actions; leading = primary safe action (Accept, Turn on, Mark read), trailing = destructive (Turn off, Remove, Revoke) which **always** confirms; full-swipe only for non-destructive actions; `touch-action: pan-y` so vertical scrolling is never captured | ⋯ menu |
| Pull to refresh | every list in standalone PWA and on platforms without native PTR | custom overscroll indicator; `overscroll-behavior-y: contain` on panes | Refresh button in toolbar / pull-down hint |
| Pinch to zoom | whole app (never disabled — no `user-scalable=no`); in-app pinch in avatar crop, QR (ACC-05) and JSON/diff viewers | native page zoom remains available everywhere | zoom buttons in viewers |
| Drag from grab handles | pinned tile reordering, sheet resize, pane dividers | drag only starts from a visible handle (`touch-action: none` on the handle only), after a 150 ms hold; list scrolling is never blocked | "Move up/down" menu items |
| Edge swipe | compact back navigation; medium: open sidebar from leading edge | uses browser/OS back where present; in standalone PWA, custom leading-edge swipe | back button / sidebar button |

### 5.3 Keyboard

macOS-style modifiers are shown as `⌘` on Apple platforms and `Ctrl` elsewhere (detect with `navigator.userAgentData.platform` / fallback). Shortcuts never override browser-reserved combos (`⌘/Ctrl+1…9, N, T, W, R, L, D`).

| Shortcut | Action |
|---|---|
| `⌘/Ctrl+K` | Command palette |
| `/` | Focus search in the current pane |
| `?` | Keyboard shortcuts dialog |
| `G` then `H` / `I` / `O` / `A` / `B` / `S` / `U` / `X` / `R` / `L` / `P` / `K` / `W` / `T` | Go to Home / Inbox / Orgs / Account / Billing / Settings / Users / Access / Roles / Audit Log / Applications (aPps) / API Keys / Webhooks / Tenant |
| `Shift+⌘/Ctrl+U` | Mark all as read (Inbox) |
| `C` | Create (Invite user, New role, Create key… per screen) |
| `E` | Edit selected |
| `J` / `K`, `↑` / `↓` | Move selection; `Enter` open; `Space` preview |
| `Shift+↑/↓`, `⌘/Ctrl+A` | Extend / select all |
| `Delete` / `⌫` | Destructive action on selection (always confirms) |
| `⌘/Ctrl+Enter` | Submit form / confirm dialog |
| `Esc` | Close preview, menu, sheet; clear selection |
| `[` | Collapse / expand sidebar |
| `⌘/Ctrl+Z` | Undo last access toggle (within its toast window, not while typing) |
| `⌘/Ctrl+C` on a focused ID | Copy ID |

All shortcuts are listed in the `?` dialog and in the palette next to their commands; single-key shortcuts are disabled while typing and can be turned off in SET-01 (WCAG 2.1.4).

## 6. System preferences (live)

The UI follows the person's system settings automatically and **updates live** when they change — no reload.

| Preference | Source | Live update | Effect |
|---|---|---|---|
| Light / dark | `prefers-color-scheme` + `color-scheme` meta | `matchMedia(...).addEventListener('change')` | theme tokens switch (`theme/themes.css`, §7c); profile `theme` setting overrides when not `system` |
| Reduced motion | `prefers-reduced-motion` | change listener | View Transitions become cross-fades or none; sheets don't slide; no parallax/auto-animations |
| High contrast | `prefers-contrast: more`, `forced-colors: active` | change listener | stronger borders/focus rings; in forced colors, use system colors (`Canvas`, `CanvasText`, `Highlight`, `ButtonText`) and keep status icons visible |
| Reduced transparency | `prefers-reduced-transparency` | change listener | no translucent/blurred surfaces |
| Language | `navigator.languages` | `languagechange` event | re-render with the new locale when profile `locale` = "Use device language"; lazy-load message bundle |
| Region & date format | `Intl.Locale` (region subtag), `Intl.DateTimeFormat().resolvedOptions()` | with `languagechange` | dates, times, numbers, first day of week (`Intl.Locale#weekInfo` where supported) |
| Time zone | `Intl.DateTimeFormat().resolvedOptions().timeZone` | checked on `visibilitychange` and every 60 s | all times re-render; profile `timezone` overrides when set |
| Text size | browser zoom / root font-size | inherent (rem units) | layout reflows; no fixed px font sizes |
| Data saver | `navigator.connection.saveData`, `prefers-reduced-data` | change listener | no prefetching, smaller avatar images |

The profile settings (SET-01) offer **"Use device setting"** as the default for language, time zone and appearance.

## 7. Progressive Web App

| Capability | Implementation | Notes |
|---|---|---|
| Installable | Web App Manifest: `name` "Substratal", `display: standalone`, `display_override: ["window-controls-overlay","standalone"]`, `start_url: "/?source=pwa"`, `scope: "/"`, maskable + monochrome icons, `shortcuts` (Home, Users, API Keys, Audit Log), `screenshots` (narrow + wide), `id` | custom install prompt in Account menu (`beforeinstallprompt`); iOS shows "Add to Home Screen" instructions |
| Window controls overlay (desktop) | toolbar draws into the title bar area via `env(titlebar-area-*)` | gives the macOS-like unified toolbar on desktop installs |
| Offline | Service worker: precache app shell + route chunks; runtime **stale-while-revalidate** for catalog, own entitlements, own profile/settings (CLIENT-ARCHITECTURE §5) stored in Cache Storage/IndexedDB; admin data **network-only**; offline fallback page | APP-01 works read-only offline; writes disabled with "Offline" caption (no write queue) |
| Updates | new SW waits; toast "A new version is available — Reload"; reload on next navigation if idle | never interrupt a form |
| Notifications | **Web Push as `app_hub`** — see §7a | iOS/iPadOS requires the PWA to be installed (16.4+) |
| Unread badges | Badging API `navigator.setAppBadge(unread_count)` from `GET /users/me/notifications/unread-count` (also from the service worker on push); `clearAppBadge()` at zero; fallback: `(3)` prefix in `document.title` + favicon dot | — |
| Native-style sharing | Web Share API: share an app link, an org invite link, a user/route link, export files (`navigator.share({files})` when `canShare`); fallback: copy link + toast | — |
| Share target | manifest `share_target` (POST, files) accepts CSV/TXT/vCard → opens bulk invite review (USR-02 / ORG-04) | Chromium-based browsers / Android |
| File handling | manifest `file_handlers` for `.csv` (invite lists) and `.json` (settings schema) | desktop Chromium |
| Protocol handler | `navigator.registerProtocolHandler('web+substratal', '/open?u=%s')` | optional |
| Periodic refresh | Periodic Background Sync (where granted) refreshes inbox inputs every ≥ 12 h | otherwise refresh on open |

## 7a. Web Push (as `app_hub`)

| Step | Implementation |
|---|---|
| Key | `app_hub`'s `web_push` credential `vapid_public_key` (public) is baked into the build config per environment; the VAPID pair can't rotate in place, so a key change ships with a release |
| Permission | From the NTF-04 primer (button click) → `Notification.requestPermission()`; never on load. Safari iOS/iPadOS: only in the installed PWA |
| Subscribe | `registration.pushManager.subscribe({userVisibleOnly: true, applicationServerKey})` → `POST /users/me/devices {application_id: "app_hub", provider: "web_push", subscription: {endpoint, keys: {p256dh, auth}}, name: "{Browser} on {OS}", app_version}` → store the `dvc_` id in IndexedDB (not secret) |
| Rotate | `pushsubscriptionchange` in the service worker → re-subscribe → `PATCH /users/me/devices/{id} {subscription}` (through the BFF-backed session) |
| Receive | SW `push` event: parse payload (`title`, `body`, `substratal` block, `category`) → `showNotification(title, {body, tag: collapse_key, data, actions})` → `setAppBadge` from a fresh `unread-count` |
| Click | SW `notificationclick` → `PATCH …/notifications/{id} {state:read}` → focus an existing client and `postMessage` the route, or `clients.openWindow(route)`; action buttons where supported (Chromium) |
| Turn off | NTF-03 → `subscription.unsubscribe()` + `DELETE /users/me/devices/{id}` |
| Unsupported | Without permission or support (e.g. iOS Safari in a tab): inbox and badges still work by polling on `visibilitychange` and every 60 s |

## 7b. Brand, icons & manifest

Assets: `brand-assets/public/` mirrors the deployed web root (see `shared/brand/BRAND.md`).

| Item | File | Wiring |
|---|---|---|
| Favicons | `favicon.svg` (small mark; the ground line turns white under `prefers-color-scheme: dark`), `favicon.ico` (16/32/48) | `head.snippet.html` |
| Home-screen / install | `apple-touch-icon.png` (180, opaque), `icons/icon-{192,512}.png` (any), `icons/maskable-{192,512}.png`, `icons/monochrome-512.png` | Merge `manifest.icons.json` into `manifest.webmanifest`: `theme_color #15132E`, `background_color #E8E8F7` |
| Safari pinned tab | `safari-pinned-tab.svg` | `<link rel="mask-icon" color="#1A1838">` |
| Web Push | `icons/icon-192.png` as `icon`, `icons/badge-96.png` as `badge` (monochrome) | `push-notification.snippet.js` |
| Social share | `og-image.png` (1200×630) | `og:image` |
| Logo in UI | `brand/lockup-inline*.svg` (sidebar header, 24 px tall), `brand/lockup-horizontal*.svg` (AUTH-01) | `<img src="/brand/lockup-inline.svg" alt="Substratal Apps">`. Switch to `-on-dark` with `<picture><source media="(prefers-color-scheme: dark)">` or the theme class. The collapsed sidebar and tablet drawer rail use `brand/mark.svg`. |


## 7c. Theming

`theme/themes.css`, `theme/themes.ts`, `theme/head-snippet.html`.

- All component colours are `var(--sub-*)` roles.
- `applyTheme()` sets `data-scheme`, `data-light-theme` and `data-dark-theme` on `<html>`, follows `prefers-color-scheme` in *Match system*, and updates `<meta name="theme-color">`.
- Put the inline head script first so there is no flash of the wrong theme.
- `forced-colors: active` hands colours to the OS.
- The PWA manifest keeps `theme_color` = Forest Floor background and `background_color` = Moss Paper background for the splash screen.
- SET-01: radiogroup swatch grid (5 columns desktop, 3 phone).

## 8. Platform features used

| Feature | API | Where | Fallback |
|---|---|---|---|
| Paste images & files | `paste` event `clipboardData.files`; Async Clipboard `read()` | ACC-01 avatar, APL-02 icon, USR-02/ORG-04 (paste a list of emails or a CSV file) | file picker |
| Copy (sensitive) | `navigator.clipboard.writeText`; clear after 60 s if unchanged (`readText` compare where permitted) | IDChip, KEY-03 | select-all text |
| System share sheet | Web Share API | see §7 | copy link |
| Camera | `<input type=file accept="image/*" capture="user">` on touch; `getUserMedia` capture dialog on desktop | ACC-01 avatar | file picker |
| File pickers | File System Access API `showOpenFilePicker` / `showSaveFilePicker` (Chromium); `<input type=file>` / download link elsewhere | exports, imports, recovery codes save | download |
| Passkey sign-in | WebAuthn with **conditional UI** (`autocomplete="username webauthn"`, `mediation: 'conditional'`) | AUTH-01 | password — **gated** on flag `passkeys` until the API ships WebAuthn (OPEN-QUESTIONS #4) |
| Password manager | correct `autocomplete` tokens; Credential Management `PasswordCredential` store after sign-in (Chromium) | AUTH-* | — |
| One-time codes | `autocomplete="one-time-code"`; WebOTP not used (TOTP, not SMS) | AUTH-02, ACC-05/06 | — |
| Smooth view transitions | View Transition API (same-document, `document.startViewTransition`) with shared element names on tiles/rows → detail headers; cross-document not needed (SPA) | list → detail, tab switches, sheet open | instant swap |
| Popover / dialog | native `popover` attribute, `<dialog>` top layer, CSS anchor positioning | menus, previews, dialogs | positioned via JS (Floating UI) |
| Screen wake / idle | Idle Detection **not** used; auto-lock after 15 min inactivity via in-page timers | admin screens | — |
| Vibration | `navigator.vibrate` for long-press feedback (Android Chrome) | touch | none |

## 9. Accessibility (web specifics)

- WCAG 2.2 AA. Landmarks (`nav`, `main`, `aside` for inspector), one `h1` per route, skip link to main, route-change focus to the heading + polite announcement of the page title.
- ARIA Authoring Practices for: grid (tables with row/cell navigation), listbox (lists with multi-select), tabs, menu/menubar, dialog/alertdialog, combobox (palette, selects), switch, treegrid not used.
- Target size ≥ 24 × 24 CSS px everywhere (2.5.8), ≥ 44 × 44 on coarse pointers.
- Dragging movements (2.5.7): every drag has a non-drag alternative (menu actions, move buttons).
- Focus not obscured (2.4.11): sticky toolbars/bottom nav use `scroll-padding`.
- Live regions: toasts `role=status`; errors `role=alert`; long operations announce completion.
- Test with VoiceOver (Safari macOS/iOS), NVDA + Firefox/Chrome, JAWS + Chrome, TalkBack + Chrome, Windows High Contrast (forced colors).

## 10. Performance

Budgets from NON-FUNCTIONAL §1: LCP ≤ 2.5 s (4G mid-tier phone), INP ≤ 200 ms, CLS ≤ 0.1, ≤ 170 KB JS gzip for the initial route.

- Route-level code splitting; admin and developer sections never load for end users.
- Preload route data and code on **intent** (hover 100 ms, focus, touchstart, palette highlight).
- Skeletons sized like final content (no layout shift); images with explicit size; `content-visibility: auto` for long settings pages.
- Optimistic updates for toggles, role assignment, mark-read; everything else waits for the server.
- Web Workers for CSV parsing (bulk invites) and JSON diffing.
- `fetchpriority=high` on Home's first tiles; HTTP caching + SW for static assets with content hashes.
- Real-user monitoring of Core Web Vitals per route (no PII).

## 11. Browser support & progressive enhancement

| Feature | Chrome/Edge | Safari (macOS/iOS) | Firefox | Fallback |
|---|---|---|---|---|
| Install / standalone | ✓ | ✓ (Add to Home Screen; macOS "Add to Dock") | — (desktop) | runs in tab |
| Web Push + badges | ✓ | ✓ when installed (iOS 16.4+) | push ✓, badges ✗ | inbox + title count |
| View Transitions | ✓ | ✓ (18+) | ✓ (recent) | instant swap |
| File System Access | ✓ | ✗ | ✗ | `<input type=file>` / download |
| Drag files out (`DownloadURL`) | ✓ | ✗ | ✗ | Download button |
| Share target / file handlers | ✓ (Android, desktop) | ✗ | ✗ | import buttons |
| Window controls overlay | ✓ | ✗ | ✗ | standard title bar |
| CSS anchor positioning | ✓ | recent | partial | Floating UI |

Build features as enhancements: detect (`'share' in navigator`, `CSS.supports(...)`, `'startViewTransition' in document`), never block a task when a feature is missing.

## 12. Screens — web-specific construction

All screens, elements and API calls are in `SCREENS.md` (web controls column). Per-screen interaction extras:

| Screen | Compact (phone) | Medium (tablet) | Expanded (desktop) |
|---|---|---|---|
| AUTH-01…07 | full-bleed form, sticky primary button above keyboard | centered card | centered card, `⌘/Ctrl+Enter` submits |
| APP-01 Home | 2-col tiles, long-press menu, swipe none, pull-to-refresh | 3–4 cols | 5–6 cols, hover preview, right-click, marquee select to pin/unpin, drag to reorder pins |
| APP-02 App info | bottom sheet (half → full) | side panel | popover from tile / inspector |
| ACC-* / SET-* | grouped rows, sheets for edits | two panes (scope list + form) | sidebar · scope list · form (macOS Settings pattern) |
| ACC-05 2FA | sheet steps; "Open in authenticator" (`otpauth://` link) | dialog | dialog with QR; pinch/zoom on QR |
| ORG-03 Members | rows, swipe Remove (confirm), FAB-style "Invite" button above bottom nav | list + detail | table with multi-select, drop CSV to invite, drag users from Users onto org in sidebar |
| USR-01 Users | card rows, search in top bar, filter sheet | list + USR-03 detail | virtualized table, resizable/sortable columns, multi-select + bulk bar, inspector, hover previews |
| ACX-01 Access | rows with switches; swipe trailing "Turn off" (confirm) | in detail pane | in detail pane; `⌘/Ctrl+Z` undo |
| ACX-05 All access | cards, filter sheet | table (condensed columns) | full table, multi-select bulk turn-off, presets "Expiring in 7/30 days" |
| ROL-02 Role | checkbox list | two panes | two panes + drag role onto user |
| APL-04 Schema | read-only preview + "Edit on a larger screen" | CodeMirror editor | editor + live SchemaForm preview side by side; drop JSON file |
| KEY-01/03 | rows; SecretReveal full-screen sheet | table | table; SecretReveal dialog |
| WHK-03 Deliveries | rows, swipe Redeliver on failed | table | table + delivery inspector |
| AUD-01/02 | rows grouped by day, filter sheet | table + side panel | table + inspector, drag event out as JSON |
| GLB-01 Palette | full-screen sheet from the search button | dialog | dialog (`⌘/Ctrl+K`) |
| NTF-01 Inbox | Inbox tab with badge; swipe Read/Archive; pull to refresh | list + detail | sidebar item with count · list · detail; multi-select; `Shift+⌘/Ctrl+U` mark all read |
| NTF-03/04 Devices & push | sheet primer → permission → subscribe | dialog | dialog; shows browser support state |
| BIL-01/02 Plan | stacked cards; Upgrade opens Stripe in the same tab and returns | two columns | two columns |
| APL-07 Push credentials | read-only summary | form dialogs | form dialogs with file drop for `.p8` / FCM JSON |

## 13. Testing checklist (web)

- [ ] Web Push in Chrome, Edge, Firefox, Safari (macOS) and installed PWA on iOS: subscribe, `pushsubscriptionchange`, click routing, badge, unsubscribe; polling fallback when permission is denied.
- [ ] Stripe Checkout/Portal round trip returning to `/billing/return` and the poll.
- [ ] Playwright projects: iPhone (WebKit, touch), Pixel (Chromium, touch), iPad (WebKit, touch + keyboard), desktop Chrome/Edge/Firefox/Safari.
- [ ] Every flow in UX-FLOWS with mouse only, keyboard only, touch only, and screen reader.
- [ ] Resize from 320 px to 2560 px on every archetype without losing state.
- [ ] Toggle each system preference (dark, reduced motion, forced colors, language, time zone) while the app is open — UI updates without reload.
- [ ] Install, go offline, reopen: Home renders read-only; writes disabled with explanation.
- [ ] Push, badge, share, share-target, file handlers on the browsers that support them; fallbacks on those that don't.
- [ ] Lighthouse/Web Vitals budgets on a throttled mid-tier phone; axe-core zero violations.
