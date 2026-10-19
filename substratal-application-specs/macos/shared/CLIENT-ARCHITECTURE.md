# Client Architecture & API Integration

How every Substratal Hub client (macOS, Windows, iOS, Android, Linux, Web) talks to the Platform API (spec v1.4.0). These rules are identical everywhere; each `PLATFORM-SPEC.md` names the native libraries that implement them.

Spec references: [Conventions](https://compositecode.github.io/substratalapps.com/api-reference/conventions/), [Auth](https://compositecode.github.io/substratalapps.com/api-reference/auth/), [Access Control](https://compositecode.github.io/substratalapps.com/access-control/), [Push Notifications](https://compositecode.github.io/substratalapps.com/api-reference/notifications/), [Billing](https://compositecode.github.io/substratalapps.com/api-reference/billing/), [Non-Functional Requirements](https://compositecode.github.io/substratalapps.com/non-functional-requirements/).

## 1. Layers

```
┌───────────────────────────────────────────────────────────────┐
│ UI (native views)  — screens from SCREEN-SPECS                │
├───────────────────────────────────────────────────────────────┤
│ Presentation state (view models / stores): screen state       │
│   machines idle → loading → loaded/empty/error; Capabilities  │
├───────────────────────────────────────────────────────────────┤
│ Domain services: Session · Directory · Access · Roles · Orgs  │
│   · Apps · Settings · Developer · Billing · Audit ·           │
│   Notifications (inbox, badge) · Push (device registration,   │
│   incoming push handling)                                     │
├───────────────────────────────────────────────────────────────┤
│ API client: auth, retries, idempotency, ETags, pagination,    │
│   error decoding, rate limits                                 │
├───────────────────────────────────────────────────────────────┤
│ Local store: secure credentials · response cache · device id  │
│   · preferences                                               │
└───────────────────────────────────────────────────────────────┘
```

**Generate the API client from `openapi.yaml`** for each platform: Swift (`swift-openapi-generator`), C# (Kiota or NSwag), Kotlin (`openapi-generator` + OkHttp/Ktor), Rust/Python for Linux (`openapi-generator`), TypeScript for Web (`openapi-typescript` + `openapi-fetch`). Screens never call HTTP directly.

## 2. The Hub is a first-party Application

To receive push notifications the Hub must own devices, and a Device belongs to an Application. So the Hub is registered in the catalog as **`app_hub`** (slug `hub`):

| Field | Value | Why |
|---|---|---|
| `access_tier` | `free` | Every User gets a platform-managed `free` Entitlement at signup, which device registration requires (`allow()` must pass). |
| `receives_platform_notifications` | `true` | Security and account alerts push to Hub devices. |
| `visibility` | `internal` | Not shown as a launchable tile; APP-01 hides it. |
| Push credentials | APNs (topic = the shared iOS/macOS bundle id), FCM (Android), Web Push (VAPID) | Configured once by staff in APL-07. |

The Hub still authenticates with **platform sessions** (it never mints app tokens for itself). If an admin disables someone's `app_hub` access, registration returns `403 entitlement_required`; the Hub then runs in inbox-only mode (no push) and says so in NTF-03.

## 3. Session lifecycle

| Token | Where it lives | Lifetime | Notes |
|---|---|---|---|
| Platform access token (JWT RS256) | Memory only | 15 min | `Authorization: Bearer` on every call |
| Platform refresh token (opaque, rotating) | OS secure store (Keychain, Credential Locker/DPAPI, Keystore, libsecret); Web: HttpOnly cookie via BFF | per spec | Persist the rotated token atomically before using the new access token |
| App token (per-app JWT) | Never stored | 5 min | Minted at launch only (APP-03) |
| MFA token | Memory | `expires_in` (~300 s) | Discard on success or expiry |
| Push device id (`dvc_…`) | Local preferences (not secret) | until deleted | Used to PATCH token rotations and to DELETE on "turn off" |

Rules:
1. **Single-flight refresh.** On `401 unauthenticated`, pause the queue, call `POST /auth/token/refresh` once, replay. Never refresh in parallel — reuse of a rotated token revokes the session and raises a `security.session_reuse_detected` alert.
2. **Proactive refresh** 60 s before expiry while in the foreground.
3. `401` on refresh or `session_revoked` → wipe credentials, show GLB-03 "Session ended", return to AUTH-01 keeping the destination.
4. **Sign out** = `POST /auth/logout`, then wipe secure store, cache, inbox cache and stored device id. The server deactivates this session's push devices (`session_revoked`); signing in again re-registers.
5. One account per device profile in v1. `test_mode` sessions (developer builds only) are stored separately with a persistent TEST indicator.

## 4. Request conventions

| Concern | Client behavior |
|---|---|
| Base URL | `https://api.substratalapps.com/v1` (configurable: prod / staging / local devserver) |
| IDs | Opaque prefixed strings (`usr_ app_ org_ ent_ role_ key_ evt_ sbs_ dvc_ pcr_ ntf_ ndl_ tnt_`). Only the prefix is ever interpreted (palette routing). `me` for `{id}` with a user token. |
| Idempotency | Generate a UUID v4 **when a create form opens**; send `Idempotency-Key` on submit and every retry. **Required** on personal and org entitlement grants and both Checkout endpoints; send it on every create POST, including `POST …/notifications`. Handle `idempotency_key_reused` (422) and `idempotency_key_in_flight` (409 → retry after 1 s). |
| Concurrency | Keep `ETag` from single-resource GETs and writes; send `If-Match` on PATCH/DELETE of Entitlements, Roles, Settings, AppSettings, Profile, Organizations, Users, Organization subscription coverage. On `409 version_conflict`: refetch, then **Reload** or **Re-apply**. |
| Pagination | Cursor: `limit` (50 desktop / 25 compact) + `cursor`; stop when `page.has_more` is false. Infinite scroll on touch; virtualized lists + "Load more" on desktop. Keep loaded pages on back-navigation. |
| Filtering | List endpoints exclude terminal states (revoked entitlements, inactive devices, archived notifications) unless `status=…` or `include_inactive=true` is passed. |
| Errors | Decode `{error:{code,message,details}}`; map `code` → COPY-AND-STATES; `details[].field` → form fields. Raw `message` only inside a "Details" disclosure with the request id. |
| Rate limits | 100 req/min per session; plus 600 sends/min per key and 60 notifications per recipient per app per hour; 10 device tests per device per hour. On `429` honor `Retry-After` and show a countdown; back off background polling ×2 up to 5 min. |
| 501 | `not_implemented` → turn the feature's flag off for the session, log once, never show an error. |
| 503 | Reads auto-retry after `Retry-After`; writes never auto-retry (GLB-03 banner). `billing_unavailable` is a 503 on billing endpoints. |
| Timeouts | 10 s connect / 30 s read; reads retry twice with jitter; writes retry only with an Idempotency-Key. |
| Headers | `User-Agent: Substratal-{platform}/{version} ({os})` (the API shows it in Sessions); `Accept-Language` from the active locale. |

## 5. Capabilities (permission gating)

Computed on sign-in and on every foreground refresh:

```
GET /users/me                      → status, mfa_enabled, email_verified, plan, plan_sources
GET /users/me/roles                → platform role ids (+ implicit)
GET /roles?scope=platform          → role → permission keys (cache 1 h; 403 = no admin permissions)
GET /applications?owned=true       → owned apps → app-owner rights
GET /tenants                       → owned Tenant(s)
GET /organizations                 → memberships + my_role (org_admin → subscription rights)
```

Exposes `canListUsers`, `canManageUsers`, `canManageEntitlements`, `canManageRoles`, `canViewAudit`, `canManageApplications`, `canManageApiKeys`, `canManageWebhooks`, `canManageTenants`, `canManageBilling`, `canSendNotifications`, `ownsApps[]`, `ownsTenants[]`, `adminOfOrgs[]`, `plan`, `planSources[]`. Navigation and every action read only from `Capabilities`; a `403` triggers a background refresh. The server is always the authority.

## 6. Push notifications and the inbox

### 6.1 What the Hub receives

| Kind | In the Hub inbox (NTF-01) | Pushed to Hub devices |
|---|---|---|
| Platform alerts (`security.*`, `account.*`) | yes | yes — `app_hub` opts in |
| Notifications sent by `app_hub` itself | yes | yes |
| Notifications from other Applications | yes (the User sees everything addressed to them) | no — they push to that app's own installs |

### 6.2 Provider per platform

The API supports **APNs, FCM and Web Push** only.

| Hub app | Provider | Notes |
|---|---|---|
| iOS / iPadOS | `apns` | `environment: sandbox` for development builds, `production` for TestFlight/App Store |
| macOS | `apns` | Same bundle id as iOS (Universal Purchase) so the single `app_hub` APNs credential's `topic` covers both — see OPEN-QUESTIONS #3 |
| Android | `fcm` | Firebase project for `app_hub` |
| Web (PWA) | `web_push` | VAPID public key from `app_hub`'s credential, shipped in the web build config (it's public); iOS/iPadOS Safari only when installed to the Home Screen |
| Windows | none | No WNS in the API. Poll `unread-count` + new items and raise **local** app notifications; taskbar badge. See OPEN-QUESTIONS #2 |
| Linux | none | No push service on Linux desktops. Poll and raise local `GNotification`s |

### 6.3 Device lifecycle (iOS, macOS, Android, Web)

1. **Ask at the right moment** (NTF-04): after sign-in on mobile at the first meaningful moment, or when the person turns notifications on — never before sign-in.
2. On OS permission granted: obtain the token/subscription → `POST /users/me/devices {application_id: "app_hub", provider, token | subscription, environment, name, app_version}` → store the returned `dvc_` id.
3. **Re-register on launch at most once a day** (keeps `stale` deactivation at bay, re-attaches to the current session after sign-in).
4. **Token rotation** (APNs device token change, FCM `onNewToken`, `pushsubscriptionchange`) → `PATCH /users/me/devices/{deviceId}` with the new token.
5. **Turn off on this device** → `DELETE /users/me/devices/{deviceId}` and stop re-registering until turned on again. The global preference is `Settings.notifications.push` (SET-01); per app via AppSettings override (SET-02).
6. **Sign out**: the server deactivates the session's devices; the client also forgets the device id.
7. OS permission revoked → nothing to call; the provider reports the token invalid on next send and the server deactivates it. Settings shows "Blocked — Open system settings".
8. Max 20 active devices per User per app: older ones become `inactive (limit)` — nothing for the client to do.

### 6.4 Handling an incoming push

- Read the `substratal` block: `notification_id`, `application_id`, `source`, `category`, `action_url`.
- **Foreground:** show an in-app banner/toast (don't duplicate a system banner), refresh `unread-count`, prepend to the inbox if visible.
- **Tap:** `PATCH /users/me/notifications/{id} {state: read}`, then route: Hub route in `action_url` → navigate; other URL → open externally; none → NTF-02.
- **Action buttons** by category (COPY-AND-STATES "Push notification text"): e.g. `account.organization_invitation` → **Accept** calls `POST /organizations/{id}/members/me/accept` from the notification action (requires unlock/auth on mobile).
- Pushes are prompts, not truth: always refetch from the API. No ordering guarantee; `collapse_key` already collapses on device.
- Content is visible on lock screens — the Hub shows only what the API sent; it never adds details to a notification.

### 6.5 Badges and unread counts

`GET /users/me/notifications/unread-count` on launch, on foreground, after any push, after marking read, and every 60 s while visible (Windows/Linux also in the background while running). Badge = total `unread_count` (app icon, Dock, taskbar, PWA badge, Inbox nav item). Per-app counts drive the Inbox app filter.

### 6.6 Notification channels / categories

- iOS/macOS: register `UNNotificationCategory` per category with actions; `security.*` uses interruption level time-sensitive (requires the Time Sensitive capability).
- Android: channels **Security alerts** (high importance), **Account & invitations** (default), **App updates** (default); map by category prefix.
- Web/Windows/Linux: actions via the Notifications API / app notification buttons where supported.

## 7. Caching & offline

| Data | Cache | Freshness |
|---|---|---|
| Capabilities, own User/Profile/Settings, plan | persistent | launch + foreground |
| Applications catalog | persistent | 1 h, or pull-to-refresh |
| Own entitlements | persistent | launch, foreground, 60 s while APP-01 visible |
| Inbox (first page) | persistent, encrypted | launch, foreground, on push; 90-day retention mirrors the API |
| Subscription | memory | on BIL screens; poll after checkout |
| Admin lists (users, access, audit, other people's notifications) | memory only | per visit |
| Secrets, app tokens, push credential key material | never | — |

Offline: Home and Inbox render read-only from cache; launch disabled (needs a fresh app token). Writes require connectivity — no offline write queue (access changes must be immediate). Clear everything on sign-out.

## 8. Launching apps (APP-03)

- **Embedded (MVP):** `POST /auth/app-tokens {application_id, organization_id?}` → hand the 5-min token to the app per its integration contract.
- **Hosted / PKCE (Phase 3):** generate verifier/challenge (S256) on device, `POST /auth/oauth/authorization-codes`, open `redirect_uri?code&state` with the platform auth-session browser; validate `state`. The hosted sign-in page itself ships with the Web Hub.

## 9. Billing hand-off

- Checkout and Portal are Stripe pages opened in the system browser (or auth-session browser on mobile). `success_url` / `cancel_url` / `return_url` are `https://dashboard.substratalapps.com/billing/return?…` (or `/orgs/{id}/subscription?…`), which Universal/App Links route back to the native app.
- Returning to `success_url` doesn't mean coverage started: poll `GET …/subscription` every 2 s for up to 30 s until `plan = standard`, then refresh Capabilities and entitlements (new standard apps appear on Home).
- In-app purchase rules for iOS/Android are an open policy question (OPEN-QUESTIONS #10); upgrade buttons are flag-gated per store.

## 10. Feature flags

Remote-configurable, also folded into `Capabilities`:
`sso`, `passkeys`, `organizations`, `org_grants`, `app_profiles`, `app_settings`, `app_roles`, `webhooks`, **`notifications`** (inbox, devices, NTF-*), **`push`** (device registration per platform), **`push_credentials`** (APL-07), **`send_notifications`** (NTF-06), **`billing`** (BIL-01..03), `billing_upgrade_ios`, `billing_upgrade_android`, `tier_changes`, `hosted_launch`, `app_registration_self_service`, `audit_export`.

A `404`/`501` from a gated endpoint turns its flag off for the session.

## 11. Security checklist

- TLS only; OS trust store (no pinning).
- Secrets (API key secrets, webhook secrets, recovery codes, push key files being uploaded) exist only in memory; KEY-03 window protected from capture where the OS allows.
- Push key material (`.p8`, FCM JSON) is read from the picked file and sent once; never cached, logged or written to temp files.
- Clipboard marked sensitive and auto-cleared after 60 s.
- No tokens, emails, IDs, notification titles/bodies or device tokens in analytics or crash reports.
- Admin screens hidden in app-switcher snapshots; auto-lock after 15 min idle.
- Deep links and `action_url`s are untrusted input: validate before routing; emailed tokens are exchanged at once and stripped from history.

## 12. Telemetry (privacy-preserving)

Shared event names: `app_launch`, `sign_in_success`, `sign_in_mfa_required`, `sign_in_failed{code}`, `screen_view{screen_id}`, `app_tile_launch`, `access_toggle{direction,managed}`, `grant_created{source}`, `role_assigned`, `key_created{mode,intended_use}`, `secret_copied`, `palette_used{result_type}`, `push_permission{result}`, `device_registered{provider}`, `push_opened{category_prefix}`, `inbox_mark_all_read`, `checkout_started{subscriber_type}`, `checkout_completed{subscriber_type}`, `error_shown{code,screen_id}`. No PII; per-install hashed ids; opt-out in SET-01.

## 13. Themes

- **Inputs:** `Settings.theme` (light / dark / system) plus `app_hub` settings `appearance.light_theme` and `appearance.dark_theme`. Ids and defaults are in THEMES.md.
- **Launch:** apply the cached choice before first paint (web: the inline head script; native: read it from preferences before the first window), then reconcile with `GET /users/me/settings` and `GET /users/me/apps/app_hub/settings`.
- **Live changes:** follow OS light/dark changes while in *Match system*. When a choice changes on another device, apply it on the next settings refresh: launch, foreground, or `settings.updated` in the inbox stream if available.
- **Fallback:** an unknown id uses the default for its mode, and the error is never surfaced to the user. Platform high-contrast modes bypass the theme.
