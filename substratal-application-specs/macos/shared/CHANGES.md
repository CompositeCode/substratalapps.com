# Changes in this package

This package targets **API spec v1.4.0** (2026-10-18; `openapi.yaml` 1.0.0-draft.3, **118 operations**). The previous package targeted v1.2.2 (98 operations). The API changes, and what they did to the apps:

## 0a. Themes (this revision)

- Adds the **theme system**: 5 light themes (Teal Ledger, Indigo Root, **Moss Paper**, Coral Clay, Spore Pink) and 5 dark themes (Night Soil, Deep Teal, Graphite Spore, **Forest Floor**, Ember Clay). Defaults are Moss Paper and Forest Floor. Rules are in `THEMES.md`; tokens are in `themes/themes.json` and `themes/themes.csv`.
- **SET-01** now has Light theme and Dark theme pickers next to Appearance, plus a new reference screen `SET-01-appearance-themes` on every platform. The choices are stored as `app_hub` settings (OPEN-QUESTIONS #22).
- Every reference screen and mockup is rendered in **both default themes** (`screens/light/`, `screens/dark/`). Mockups preview any theme with `?mode=dark&light=<id>&dark=<id>`.
- New per-platform theme code in `<platform>/theme/`: Swift `ThemeStore` and palettes (iOS/macOS); WinUI theme dictionaries and `ThemeService` (Windows); Material 3 colour schemes (Android); libadwaita CSS (Linux); CSS variables with a TypeScript resolver (web).
- COMPONENTS adds **ThemePicker**. CLIENT-ARCHITECTURE adds §13 Themes. Theming is now in scope (PRODUCT-SPEC).

## 0. Logo standard and app icons (this revision)

- Adopted logo: **interlined root twist** mark with the `[substratal]` wordmark (JetBrains Mono, coral brackets, ink `sub`, teal `stratal`, `◆ apps` descriptor). Rules: `brand/BRAND.md`.
- Placed in every design: AUTH-01/02 sign-in (all platforms), web sidebar header, Windows title bar, notification rows for `source: platform`. Reference screens and mockups have been re-rendered.
- New per-platform kits under `<platform>/brand-assets/`: Xcode asset catalogs (AppIcon with dark/tinted, Icon Composer layers, logo imagesets, colour sets), `.icns`, Windows `.ico` and full MSIX asset set, Android adaptive/themed/notification icons as vectors and PNGs, Linux hicolor and symbolic icons with `.desktop` and AppStream branding, and the web favicon set, PWA icons (any/maskable/monochrome), Web Push badge and OG image.
- Each `PLATFORM-SPEC.md` has a new **Brand & app icon** section. COMPONENTS adds **BrandLockup**.

## 1. Push notifications and the notification inbox (API: Push Notifications, 17 new operations)

| API change | Effect on the Hub |
|---|---|
| Devices per Application (`/users/{id}/devices`), APNs / FCM / Web Push | The Hub registers itself as first-party app **`app_hub`** and registers each install (CLIENT-ARCHITECTURE §2, §6). New flow F12, screens **NTF-03 Devices & notifications** and **NTF-04 Turn on notifications**. |
| Inbox (`/users/{id}/notifications`, unread-count, mark-read) | The client-composed inbox is gone. **NTF-01 Inbox** and **NTF-02 Notification** read the real inbox; badges use `unread-count` everywhere. Inbox is a top-level destination on every platform (mobile tab bar: Home · Inbox · Orgs · Manage · Account). |
| Platform alerts (`security.*`, `account.*`) pushed to apps with `receives_platform_notifications` | `app_hub` opts in; categories get native actions (Review sessions, Accept invitation…). Flow F13. |
| `notifications.push` channel in Settings (global and per app) | Push toggles in SET-01 and SET-02. |
| Push credentials per Application (`/applications/{id}/push-credentials`) | New **APL-07 Push credentials** tab for app owners; `receives_platform_notifications` toggle on APL-03. Flow F15. |
| `notifications.send` permission and `POST /users/{id}/notifications` | New staff tool **NTF-06 Send notification**; app keys can carry confined `notifications.send` (KEY-02). |
| Support can read any inbox and devices (`users.list`), delete devices (`users.manage`) | New user-detail tab **NTF-05**. Flow F16. |
| Webhook events `device.*`, `notification.push_failed`, `push_credential.failed` | WHK-02 event list and "Push health" preset. |
| Logout / session revoke / suspend / delete deactivate devices | Copy for sign-out, suspend and "sign out everywhere" updated. |
| All Notifications operations return 501 until implemented | Everything above ships behind flags that a 501 turns off. |
| No WNS provider; no Linux push | Windows and Linux keep polling and show local notifications (OPEN-QUESTIONS #2). |

The old push-relay assumption (previous OPEN-QUESTIONS #2) is withdrawn for iOS, macOS, Android and Web.

## 2. Free / Standard pricing (API: Billing rewritten)

| API change | Effect on the Hub |
|---|---|
| Developers/Tenants are no longer billed; Tenant `plan`, `subscription_status`, `restricted` and `/tenants/{id}/usage` removed | Old "Plan & usage" developer screen removed. **BIL-04 Tenant & infrastructure** keeps tier/region and tier-change requests only, under Developer › Tenant. |
| Users have `plan` and `plan_sources`; `GET /users/{id}/subscription`; individual Checkout/Portal | New **BIL-01 My plan** under Personal › Plan & Billing (Account on mobile). Flow F10 rewritten. |
| Organization subscription with `member_scope` and Checkout/Portal | New **BIL-02 Organization subscription** tab on organizations. Flow F14. |
| Checkout success ≠ coverage; poll until `plan = standard` | **BIL-03** returns to `https://dashboard.substratalapps.com/billing/return…` and polls. |
| Applications gain `access_tier` (`free` / `standard`) | Home separates "Included with Standard" tiles with an upsell; APL-02/03 show access tier (staff-only field). |
| `402 subscription_required`, plan caps, `seat_limit` removed; `plan_limit_reached` reserved | Removed from every screen and from COPY. |
| Store policy | Standard is now a consumer subscription unlocking apps → mobile upgrade buttons default off pending policy review (OPEN-QUESTIONS #10). |

## 3. Platform-managed entitlements

| API change | Effect on the Hub |
|---|---|
| `Entitlement.source` adds `free` and `subscription`; `subscription_id` | New source labels; APP-02 and ACX-01 show "Managed by Substratal" paths. |
| Only disable / re-enable allowed; anything else `409 entitlement_platform_managed` | ACX-01/03 hide term, scope and delete for those rows; confirm copy explains. |
| Platform rows and developer grants coexist | ACX-02 no longer blocks on platform rows; ACX-05 adds a "Show platform-managed" filter. |

## 4. Smaller changes

- Audit hot window is one year for every Tenant (AUD-01 reads `Audit-Hot-Window-Start`); new audit actions `subscription.*`, `push_credential.*`, `device.deleted`.
- Seed role `superadmin` gains `notifications.send`; `billing.manage` now means people's and organizations' subscriptions.
- Signup with an invited user's email returns `409 email_taken` and re-sends the invitation (AUTH-03 copy).
- ID prefixes added: `sbs_`, `dvc_`, `pcr_`, `ntf_`, `ndl_`.
- Web host is `https://dashboard.substratalapps.com` (matches the API's billing examples).

## 5. Package changes

- 75 screens (was 68): +NTF-01…06, +APL-07, BIL screens replaced; GLB-03 Inbox folded into NTF-01; GLB-02 is now the combined **Manage** hub; GLB-03 is System states.
- 16 flows (was 11): F10 rewritten; F12–F16 added.
- New: `CHANGES.md`. Updated: every shared document and every platform spec and screen list; mockups add Inbox, Devices, My plan, and Push credentials screens.
