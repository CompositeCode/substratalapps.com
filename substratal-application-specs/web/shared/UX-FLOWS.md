# UX Flows

Screen IDs in brackets refer to `SCREEN-SPECS.md`. Flow → screens: F1 AUTH-03/04 · F2 AUTH-01/02/05/06 · F3 APP-01/02/03 · F4 ACC-04/05/06 · F5 ACC-07/08 · F6 USR-01/03, ACX-01..04 · F7 ROL-01..04 · F8 ORG-01..09 · F9 APL-02/03, KEY-02/03, WHK-02/03 · F10 BIL-01/03 · F11 AUD-01/02 · F12 NTF-04/03 · F13 NTF-01/02, ORG-09 · F14 BIL-02/03, ORG-03 · F15 APL-07, NTF-03 · F16 NTF-05/06.

Each flow lists the screens, the API calls behind each step, and the branches the UI must handle. Diagrams are Mermaid (render in GitHub, VS Code, or open `ux-flows.html` in this folder).

---

## F1 · First run: sign up → verify → home

```mermaid
flowchart TD
  A[Welcome] -->|Create account| B[Sign up form]
  A -->|Sign in| L[Sign in]
  A -.->|Opened from invite link| INV[Accept invitation]
  B -->|POST /auth/signup| C{201}
  C -->|session issued| D[Check your email screen]
  D -->|Resend · POST /auth/email/verify/resend 202| D
  D -->|deep link token · POST /auth/email/verify| E[Email verified ✓]
  E --> H[Home · My Apps]
  C -->|422 validation_failed| B
  INV -->|POST /auth/invitations/accept| H
```

- Signup returns a session immediately; unverified users reach Home with a persistent "Verify your email" banner.
- Every new account is on **Free** with access to every free app (platform-managed `free` entitlements); standard apps show as "Included with Standard".
- Password field: show strength meter, show/hide toggle, platform password-manager autofill (`textContentType=.newPassword`, `autocomplete="new-password"`).

## F2 · Sign in with MFA and session upkeep

```mermaid
flowchart TD
  L[Sign in] -->|POST /auth/login| R{response}
  R -->|session| H[Home]
  R -->|mfa challenge + methods| M[Two-factor code]
  M -->|POST /auth/mfa/verify| H
  M -->|Use a recovery code| MR[Recovery code entry] --> H
  R -->|401 unauthenticated| L
  R -->|429 rate_limited| W[Wait message w/ Retry-After] --> L
  L -->|Forgot password| FP[Email entry] -->|POST /auth/password/forgot 202| FS[If an account exists, we sent a link]
  FS -.->|deep link| RP[New password] -->|POST /auth/password/reset| SO[Signed out everywhere notice] --> L
  H -.->|access token 15 min; refresh in background| RT[POST /auth/token/refresh]
  RT -.->|401 session_revoked| L
```

- MFA challenge expires (`expires_in` ≈ 300 s): show a quiet countdown; on expiry return to Sign in with message.
- Store refresh token in the platform secure store only (Keychain, Credential Locker/DPAPI, Android Keystore-backed EncryptedSharedPreferences/DataStore, libsecret).
- Offer biometric unlock to re-open the app (Touch ID/Face ID, Windows Hello, BiometricPrompt, — on Linux, OS keyring unlock) — this gates access to the stored refresh token; it is not a server auth factor.

## F3 · Launch an app (the MVP happy path)

```mermaid
flowchart TD
  H[Home · My Apps] -->|GET /applications + GET /users/me/entitlements| G[Grid of tiles w/ resolved status]
  G -->|tap active tile| X{launch mode}
  X -->|embedded| T[POST /auth/app-tokens] --> O[Open launch_url with app token<br/>in-app browser / system browser]
  X -->|hosted PKCE, Phase 3| P[POST /auth/oauth/authorization-codes] --> O
  G -->|disabled/expired tile| I[Info sheet: why + who to contact]
  G -->|none + public| GA[Get access → app's own page]
  G -->|long-press / right-click| CM[Context menu: App settings · App profile · Copy app ID]
```

- Tile ordering: Recently launched, then alphabetical. Pin/favorite is client-side only.
- App token TTL is 5 minutes — mint at tap time, never cache.

## F4 · Account security: enable two-factor

```mermaid
flowchart LR
  S[Account › Security] -->|Turn on| E[POST /users/me/mfa/totp] --> Q[QR + secret, Copy / open in authenticator]
  Q --> C[Enter 6-digit code] -->|POST …/totp/confirm| RC[Recovery codes — shown once]
  RC -->|Copy · Save · Print| D[Done — I've saved them]
  D --> S
  S -->|Turn off| OFF[Enter code or recovery code] -->|POST …/totp/disable| S
```

- Recovery-code screen blocks dismissal until "I've saved these" is checked.
- Sessions list: current device marked "This device"; "Sign out of all other sessions" revokes each other session (`DELETE /users/me/sessions/{id}`), which also deactivates their push devices.
- Turning 2FA off raises a `security.mfa_disabled` platform alert to every Hub device (F13).

## F5 · Delete my account / export my data

```mermaid
flowchart LR
  P[Account › Privacy] -->|Download my data| X[GET /users/me/export] --> SV[Save file dialog / share sheet]
  P -->|Delete account| W[Warning: 7-day window, what is erased] --> TY[Type DELETE to confirm] -->|POST erasure-requests| SC[Scheduled for date — signed out]
```

## F6 · Admin turns a user's access off (and back on)

```mermaid
flowchart TD
  U[Users] -->|search| UD[User detail › Access tab]
  UD -->|GET /users/:id/entitlements| AL[Per-app rows: resolved status + paths]
  AL -->|toggle off on a personal path| CF[Confirm: Turn off Timetrack for Dana? reason]
  CF -->|PATCH /entitlements/:id status=disabled| AL
  AL -->|org-seat path still active| WN[Warning: still has access via Org Acme — manage seat grant]
  AL -->|Why?| EP[GET effective-permissions → inspector sheet]
  AL -->|Grant access| GR[Grant sheet: app, source, starts/ends] -->|POST /users/:id/entitlements| AL
```

- Optimistic toggle with rollback on error; success toast "Access turned off · Undo" (Undo = PATCH back to `active`, within toast lifetime).
- If the user only has access through an organization, the switch on that row is replaced by "Via Acme (seat)" with a link to the org's seat grant.
- `DELETE` (hard remove) appears only when eligible (developer grant, no `order_id`, < 24h) as "Remove grant made in error".
- Platform-managed rows (`free`, `subscription`) only toggle off/on; the confirm explains the access comes with their plan and stays off until turned back on.

## F7 · Roles

```mermaid
flowchart LR
  RL[Roles] --> RD[Role detail: permissions checklist]
  RL -->|New role| NR[Name, scope platform/app, permissions] -->|POST /roles| RD
  RD -->|Edit, non-seed only| PATCH[PATCH /roles/:id]
  UD[User detail › Roles tab] -->|Assign role| PICK[Role picker] -->|POST /users/:id/roles/:roleId| UD
```

## F8 · Organization admin

```mermaid
flowchart TD
  O[Organizations] -->|New| NO[Name] -->|POST /organizations| OD[Org detail]
  OD --> MEM[Members tab] -->|Invite by email| INV[POST /organizations/:id/members]
  MEM -->|change role / remove| MR[PATCH / DELETE member]
  OD --> SEATS[Apps & Seats tab] -->|Grant app to org| SG[POST /organizations/:id/entitlements]
  SEATS -->|Member scope: all / allowlist / denylist| MS[PATCH /entitlements/:id member_scope]
  INVITEE[Invited user · Home banner] -->|Accept| ACC[POST members/me/accept] --> OD
  INVITEE -->|Decline| DEC[DELETE members/me]
```

## F9 · Developer: register an app, get credentials, wire webhooks

```mermaid
flowchart TD
  A[Applications] -->|Register| R[Name, slug immutable, launch URL, redirect URIs, visibility]
  R -->|POST /applications| AD[App detail: Overview · Settings schema · Roles · Keys · Webhooks · Activity]
  AD -->|Create app key| K[Name, mode live/test, use service/agent, permissions, restrict destructive]
  K -->|POST /api-keys| SECRET[Secret shown once — Copy]
  AD -->|Add webhook| W[URL + event types] -->|POST /webhooks| WS[Signing secret shown once]
  WS -->|Send test| T[POST /webhooks/:id/test] --> DL[Deliveries log]
  DL -->|failed row| RD[Redeliver]
```

- Key rotation copy: "The current secret stops working immediately." Webhook secret rotation copy: "Both secrets are valid for 24 hours."

## F10 · A person upgrades to Standard

```mermaid
flowchart TD
  H[Home: standard app tile 'Included with Standard'] --> U[Upsell sheet: every app for $6/month]
  P[Plan & Billing BIL-01] --> U
  U -->|Upgrade to Standard| C[POST /users/me/billing/checkout-sessions · Idempotency-Key]
  C -->|201 url| S[Stripe Checkout in browser]
  C -->|409 subscription_exists| PO[POST /users/me/billing/portal-sessions → Portal]
  S -->|success_url → app deep link| W[Finishing up… poll GET /users/me/subscription]
  W -->|plan = standard| D[Refresh Capabilities + entitlements → standard apps unlock on Home]
  W -->|30 s, still free| L[Taking longer than usual · check again later]
  S -->|cancel_url| X[Toast: Checkout cancelled]
  P -->|Manage billing| PO
```

- Mobile store builds show the Upgrade button only when its store flag is on (OPEN-QUESTIONS #10).
- Covered by an org and also subscribed individually → BIL-01 suggests cancelling the individual subscription in the Portal.
- `past_due` keeps access (Stripe retries ~3 weeks); BIL-01 shows "Payment issue · Update it in Manage billing".

## F11 · Audit investigation

```mermaid
flowchart LR
  AU[Audit Log] -->|filters: actor type, action, resource, date| L[GET /audit-events]
  L --> D[Event detail: actor, resource link, before/after diff, request id]
  D -->|open resource| RES[User / App / Org detail]
```

---

## F12 · Turn on push notifications (iOS, macOS, Android, Web)

```mermaid
flowchart TD
  M[Meaningful moment: first sign-in on mobile, org invite, or toggle in NTF-03/SET-01] --> PR[NTF-04 primer]
  PR -->|Not now| SN[Snooze 14 days]
  PR -->|Turn on| OS{OS permission prompt}
  OS -->|denied| BL[NTF-03 shows Blocked · Open system settings]
  OS -->|granted| TK[Get APNs token / FCM token / PushSubscription with app_hub VAPID key]
  TK --> RG[POST /users/me/devices application_id=app_hub]
  RG -->|201 or 200| ST[Store dvc_ id · This device: On]
  RG -->|403 entitlement_required| IO[Inbox-only mode explained]
  ST -.->|daily on launch| RR[POST again to refresh]
  ST -.->|OS rotates token| PT[PATCH /users/me/devices/:id]
  ST -.->|Turn off on this device| DL[DELETE /users/me/devices/:id]
  ST -.->|Send test| TS[POST …/devices/:id/test → sent / failed]
```

- Windows and Linux skip this flow: notifications come from polling and are shown locally (CLIENT-ARCHITECTURE §6.2).
- Sign-out deactivates the device server-side; signing in again re-registers it under the new session.

## F13 · Receive and act on a notification

```mermaid
flowchart TD
  EV[Platform alert or app_hub notification is sent] --> IN[Lands in inbox — always]
  EV --> PU{Device for app_hub and push on?}
  PU -->|yes| PN[System notification with actions]
  PU -->|no| IB[Badge updates on next unread-count]
  PN -->|tap| RD[PATCH state=read] --> RT{action_url}
  RT -->|Hub route| NAV[Navigate in-app]
  RT -->|other URL| EXT[Open externally]
  RT -->|none| ND[NTF-02 detail]
  PN -->|Accept invitation action| AC[POST /organizations/:id/members/me/accept]
  PN -->|Review sessions action| SE[ACC-04 sessions]
  IN --> NT[NTF-01 Inbox: filter, mark read, archive, mark all read]
```

- Foreground pushes become an in-app banner and refresh the inbox; the badge always comes from `unread-count`.
- Other apps' notifications appear in the Hub inbox but push only to those apps' own installs.

## F14 · An organization subscribes to Standard for its members

```mermaid
flowchart TD
  O[ORG-03 › Subscription tab BIL-02] -->|no subscription| SB[Subscribe the organization]
  SB --> SC[Choose who's covered: everyone / selected / everyone except]
  SC --> CK[POST /organizations/:id/billing/checkout-sessions · member_scope]
  CK --> ST[Stripe Checkout] -->|success| PL[Poll GET /organizations/:id/subscription until plan = standard]
  PL --> CV[Covered members get Standard apps at once]
  O -->|subscription in force| ED[Edit coverage]
  ED --> CF[Confirm: n gain, m lose access now; billed from next daily sync]
  CF --> PA[PATCH /organizations/:id/subscription · If-Match]
  O -->|Manage billing| PO[POST …/billing/portal-sessions → Stripe Portal]
```

- Billed users = covered active members (`quantity.current`); the invoice uses the daily-synced count (`quantity.last_synced`).
- Removing a member from the org removes their coverage immediately.

## F15 · Developer sets up push for their app

```mermaid
flowchart LR
  A[APL-03 › Push tab APL-07] --> P{Provider}
  P -->|APNs| AP[team_id, key_id, topic, .p8 file]
  P -->|FCM| FC[service account JSON]
  P -->|Web Push| WP[subject → platform generates VAPID; copy public key]
  AP & FC & WP --> POST[POST /applications/:id/push-credentials]
  POST -->|201| OK[Active]
  POST -->|422 invalid_push_credential| ER[Field error]
  OK --> T[Prove it: owner's own device test]
  OK -.->|provider rejects key| F[Status: Needs attention · push_credential.failed webhook] --> FX[PATCH new key material]
  A --> TG[Toggle platform alerts receives_platform_notifications]
```

## F16 · Support investigates "I never got the alert"

```mermaid
flowchart LR
  U[USR-03 › Notifications tab NTF-05] --> N[Find notification by category/time]
  N --> D[push.status + skip_reason + deliveries]
  D -->|push_disabled| P[Person turned push off — explain]
  D -->|no_devices| V[Devices table: inactive reasons]
  D -->|failed| E[Provider error per device]
  V -->|users.manage| X[Delete stale device]
  U --> S[Staff with notifications.send: NTF-06 send a test notification on behalf of an app]
```

## Cross-cutting behavior

| Situation | Behavior on every platform |
|---|---|
| `401 unauthenticated` mid-session | Try refresh once; on failure, return to Sign in preserving the deep link destination. |
| `403 forbidden` | Inline empty-state "You don't have permission to view this." — never a crash or blank screen. |
| `409 not_unique` / `entitlement_already_exists` | Inline field error or "Already granted — view it" link. |
| `409 version_conflict` (`If-Match` mismatch) | "This changed since you opened it." Reload / Re-apply. |
| `422 validation_failed` | Map `details[].field` to field-level errors. |
| `429 rate_limited` | Disable the action, show retry countdown from `Retry-After`. |
| `503 service_unavailable` | Non-blocking banner; queue nothing — writes are retried manually. |
| `501 not_implemented` | Feature flag off for the session; entry points hidden. |
| Incoming push | Refetch from the API; never trust push content as state. |
| Idempotency | Every create/POST sends an `Idempotency-Key` (UUID v4) generated when the form opens, reused on retry (required on grants and checkouts). |
| Pagination | Cursor based; preserve scroll position on back-navigation. |
| Optimistic updates | Allowed for toggles and role assignment only; everything else waits for server. |
