# Non-Functional Requirements (client side)

## 1. Performance budgets

| Metric | Desktop native | Mobile native | Web |
|---|---|---|---|
| Cold start → interactive Home (cached) | ≤ 1.0 s | ≤ 1.5 s (mid-tier device) | LCP ≤ 2.5 s on 4G mid-tier phone; ≤ 1.2 s repeat visit (service worker) |
| Navigation to a cached screen | ≤ 100 ms | ≤ 150 ms | ≤ 100 ms (client routing) |
| Input feedback (press, toggle) | next frame (≤ 16 ms) | next frame | INP ≤ 200 ms (p75) |
| List scroll | 60 fps (120 on ProMotion/high-refresh) | 60/120 fps, no jank | no long tasks > 50 ms during scroll |
| Layout stability | — | — | CLS ≤ 0.1 |
| Search results (server) | show skeleton at 300 ms | same | same |
| Launch app (APP-03) | token minted + handoff ≤ 1 s p75 | ≤ 1.2 s | ≤ 1 s |
| Push tap → target screen | ≤ 1 s warm, ≤ 2 s cold | ≤ 1.5 s warm, ≤ 2.5 s cold | ≤ 1.5 s (service worker focuses/opens the client) |
| Badge freshness | ≤ 60 s while running (polling) | on push + foreground | on push + visibility change |
| Return from Stripe → plan shown | ≤ 30 s (poll) | ≤ 30 s | ≤ 30 s |
| Bundle / binary | — | ≤ 40 MB download | ≤ 170 KB JS (gzip) initial route; route-split the rest |
| Memory | ≤ 250 MB with 10k-row table | ≤ 200 MB | — |

Large collections (users, audit, deliveries) must be virtualized on every platform.

## 1a. Platform behavior the clients rely on

- Push dispatch p95 < 10 s from send to first provider attempt; providers may still delay or drop — the inbox is the source of truth.
- Inbox retention 90 days; delivery details 30 days; inactive devices deleted after 30 days; devices go stale after 270 days without re-registration (clients re-register daily on launch).
- Audit hot window: 1 year for every Tenant.
- Stripe coverage changes land via webhook, usually within seconds of checkout; org quantity syncs daily at 00:15 UTC.

## 2. Reliability

- No crash rate above 0.2 % of sessions (native); no unhandled promise rejections (web).
- Every write is idempotent from the client's point of view (Idempotency-Key, If-Match).
- Network loss mid-write: the control returns to its prior state with an error; the user can retry; the retry reuses the same Idempotency-Key.

## 3. Security & privacy

See CLIENT-ARCHITECTURE §9. Additionally:
- Admin screens show other people's personal data: no persistent caching, hide in app switcher/screenshots where supported, auto-lock after 15 min idle (configurable by org policy later).
- Data export files (ACC-07, USR-03) are written only where the user chooses; no copies left in temp/cache.
- Privacy manifests / data-safety forms: collected data = email, name, user ID, device/session info, **push device tokens** (sent to Substratal and Apple/Google/browser push services, which are sub-processors), purchase history (Stripe, outside the app), usage analytics (opt-out).
- Notification content is never logged, analyzed or added to by the client; lock-screen previews follow the OS setting.

## 4. Localization

- Launch locales: en-US. Architecture must support RTL and pluralization from day one.
- Strings externalized with keys mirroring screen IDs (`ACX03.title`, `COPY.confirm.access_off.body`).
- Dates, times, numbers, lists and relative times formatted with the platform locale APIs using the profile's `locale` and `timezone` (fallback: OS). Times on admin screens show the viewer's zone, with UTC on hover/long-press.
- Pseudo-localization build (+40 % length, accented) must lay out without truncating status labels or buttons.

## 5. Accessibility

WCAG 2.2 AA plus platform guidelines — see ACCESSIBILITY.md.

## 6. Supported platforms

| App | Minimum | Primary test matrix |
|---|---|---|
| macOS | macOS 15 | 15, 26 · Apple silicon + Intel |
| Windows | Windows 10 22H2 (WinAppSDK) | Windows 11 24H2/25H2 · 100–200 % scaling · touch laptop |
| iOS/iPadOS | iOS 17 | iPhone SE, iPhone 17, iPhone 17 Pro Max, iPad mini, iPad Pro 13" with keyboard + Pencil |
| Android | API 26 (8.0) | Pixel 6a, Pixel 10, Samsung foldable (folded/unfolded), 10" tablet, ChromeOS |
| Linux | GNOME 46+ (libadwaita 1.5+) | Ubuntu 24.04/26.04, Fedora latest, KDE Plasma (Flatpak) |
| Web | Last 2 versions of Chrome, Edge, Safari, Firefox; Safari iOS 17+; Samsung Internet | phone 360–430 px, tablet 768–1180 px, desktop 1280–2560 px |

## 7. Quality gates per release

- Unit tests on domain services and Capabilities (every persona × every nav item).
- Contract tests against the API's staging environment generated from `openapi.yaml`.
- UI tests for the 11 flows in UX-FLOWS.md on every platform.
- Accessibility automated checks (XCTest a11y audits, Accessibility Insights, Espresso/Compose checks, axe-core) + manual screen-reader pass of the 11 flows.
- Performance traces against the budgets above on the lowest test device.
