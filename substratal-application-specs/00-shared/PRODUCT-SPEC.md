# Substratal Hub — Product Spec

**What we're building:** six client apps — macOS, Windows, iOS/iPadOS, Android, Linux (GNOME) and a responsive Web app/PWA — together the **Substratal Hub**: the dashboard client the API spec plans for. Each is a full client of the **Substratal Apps Platform API, spec v1.4.0 (2026-10-18), `openapi.yaml` 1.0.0-draft.3, 118 operations.**

People use the Hub to launch and manage their apps, read their notifications, choose Free or Standard, run their organizations, and — for staff and app developers — administer users, access, roles, applications, credentials, push and audit.

**Source of truth:** https://compositecode.github.io/substratalapps.com/. If this package and the API spec disagree, the spec wins; record the difference in `OPEN-QUESTIONS.md`. What changed since the previous package is in `CHANGES.md`.

**In scope:** visual design. That means the logo and app icons (`brand/BRAND.md`) and the theme system: 5 light and 5 dark themes with defaults Moss Paper and Forest Floor, chosen by the user in SET-01 (`THEMES.md`). This package defines structure, behavior, content, theming and native construction; every reference screen is rendered in both default themes.

## 1. Personas

| Persona | Who | Rights | Main screens |
|---|---|---|---|
| **End user (Free)** | Every account | Own account; free apps | AUTH-*, ACC-*, APP-*, SET-*, NTF-01..04, BIL-01, ORG-01/09 |
| **Subscriber (Standard)** | Pays $6/month for themselves, or covered by an organization | Every standard app | + BIL-01 manage billing |
| **Org admin** | `OrganizationMembership.role = org_admin` | Members, seat grants, the org's Standard subscription and who it covers | ORG-*, BIL-02/03 |
| **App owner (developer)** | Owns an Application (and therefore a Tenant) | App configuration, push credentials, app keys, webhooks, grants for own app. **Not billed** — Tenants have no plan. | APL-*, KEY-*, WHK-*, BIL-04, ACX-* (confined) |
| **Support** | Seed role `support` | `users.list`, `entitlements.manage`, `audit.view`, `tenants.manage`, `billing.manage` | USR-*, ACX-*, NTF-05, AUD-*, BIL-04, BIL-01/02 (read) |
| **Billing admin** | Seed role `billing_admin` | `billing.manage`, `billing.refund`, `audit.view`, `users.list` | BIL-01/02 (read + portal), USR-01/03, AUD-* |
| **Superadmin** | Seed role `superadmin` | Everything, incl. `notifications.send` | All, incl. NTF-06 |

## 2. Domains and screens (75)

| Domain | Purpose | Screens |
|---|---|---|
| D1 Sign-in & Identity | Get a session | AUTH-01 Sign in · AUTH-02 Two-factor · AUTH-03 Create account · AUTH-04 Verify email · AUTH-05 Forgot password · AUTH-06 New password · AUTH-07 Accept invitation |
| D2 Account & Security | Own identity and safety | ACC-01 Account & profile · ACC-02 Change email · ACC-03 Change password · ACC-04 Security · ACC-05 Turn on 2FA · ACC-06 Turn off 2FA · ACC-07 Privacy · ACC-08 Delete account |
| D3 My Apps | See and launch apps | APP-01 Home · APP-02 App info · APP-03 Launch · APP-04 App profile · APP-05 Past apps |
| D4 Settings | Preferences, incl. push on/off | SET-01 Global · SET-02 Per app |
| D5 Organizations | Teams and seat grants | ORG-01 List · ORG-02 New · ORG-03 Members · ORG-04 Invite · ORG-05 Member actions · ORG-06 Apps & seats · ORG-07 Grant / scope · ORG-08 Settings · ORG-09 Invitation |
| D6 Users | Account administration | USR-01 Directory · USR-02 Invite · USR-03 Overview · USR-04 Sessions · USR-05 Suspend |
| D7 Access | The on/off switch | ACX-01 User access · ACX-02 Grant · ACX-03 Turn off/edit · ACX-04 Why? · ACX-05 All access |
| D8 Roles & Permissions | What people can do | ROL-01 Roles · ROL-02 Detail · ROL-03 New · ROL-04 User roles |
| D9 Applications | Developer app config | APL-01 List · APL-02 Register · APL-03 Overview · APL-04 Settings schema · APL-05 Permissions & roles · APL-06 Review · **APL-07 Push credentials** |
| D10 Developer credentials | Keys and webhooks | KEY-01 Keys · KEY-02 Create · KEY-03 Secret reveal · KEY-04 Key detail · WHK-01 Webhooks · WHK-02 Add/edit · WHK-03 Deliveries · WHK-04 Delivery |
| D11 Plan & Billing | Free/Standard for people and orgs; tenancy | **BIL-01 My plan · BIL-02 Organization subscription · BIL-03 Checkout/portal · BIL-04 Tenant & infrastructure** |
| D12 Audit Log | Accountability | AUD-01 Log · AUD-02 Event |
| **D13 Notifications** | Inbox, push devices, staff tools | **NTF-01 Inbox · NTF-02 Notification · NTF-03 Devices & notifications · NTF-04 Turn on notifications · NTF-05 User notifications & devices (support) · NTF-06 Send notification (staff)** |
| G Shell | Cross-cutting | GLB-01 Command palette · GLB-02 Manage hub · GLB-03 System states |

## 3. Core ideas every screen must respect

1. **Is the app on for this person?** → Entitlement status (`active`, `scheduled`, `disabled`, `expired`, `revoked`, `none`) — StatusBadge + AccessSwitch.
2. **If on, what can they do?** → Roles → Permissions — role list / permission checklist.
3. **Where does access come from?** A person can have several paths to one app: **platform-managed** (`free` for free apps, `subscription` while Standard covers them) and **developer grants** (`purchase`, `trial`, `admin_grant`, `org_seat`). They coexist. The UI shows the resolved status first and the paths one tap/click away. Platform-managed rows can only be turned off and back on — never edited, revoked or deleted.
4. **Who pays?** People, not developers. Free costs nothing; **Standard is $6 per user per month**, bought by a person for themselves or by an organization for the members it chooses. Payment always happens in Stripe in the browser.
5. **Notifications have two halves.** The **inbox** (API, always complete, 90 days) and **push** (best effort, per device). The Hub shows a unified inbox of every app's notifications plus Substratal's security and account alerts; it pushes only its own and platform alerts (it is the first-party app `app_hub` that opts into platform alerts). Push is never used to grant or revoke anything.

## 4. Phase gating

Build every screen; ship behind flags (CLIENT-ARCHITECTURE §8):

| Phase | Screens / features |
|---|---|
| **MVP** | D1 (no SSO/passkeys), D2, APP-01/02/03/05, SET-01, D6, ACX-01/02/03/05, ROL-* (platform scope), APL-01/02/03, KEY-*, D12, GLB-* |
| **Phase 2** | APP-04, SET-02, ACX-04, app-scoped roles, APL-04/05, WHK-*, **D13 Notifications + APL-07**, **D11 Billing (Free/Standard)**, ORG-* (organizations and org subscriptions), BIL-04 tier changes |
| **Phase 3** | Hosted launch (PKCE) and the hosted sign-in page (served by the Web Hub), SSO, APL-06 review + self-service registration, audit export |

Notifications operations currently return `501 not_implemented` until the reference implementation ships them — the client treats 501 as "flag off".

## 5. Cross-platform consistency contract

Every app matches on: screen inventory, IDs and routes; navigation sections and order; copy; status semantics (`status-semantics.json`); behavior contracts (AccessSwitch, SecretReveal, confirms, Undo windows, idempotency, conflicts, push handling); and every gesture/shortcut having a visible equivalent. Apps **differ** where their platform's conventions differ, as defined in each `PLATFORM-SPEC.md` — including which push service is available (§6 of CLIENT-ARCHITECTURE).

## 6. Acceptance criteria (each app)

- [ ] All 75 screens implemented or flag-gated per phase; each reachable by route/deep link.
- [ ] Every API operation in `API-SCREEN-MAP.csv` reachable (118/118; `/auth/oauth/token` is called by launched apps, not the Hub).
- [ ] Capabilities gating matches INFORMATION-ARCHITECTURE for all personas (test account per persona, plus a Free and a Standard end user, and an org-covered user).
- [ ] Loading, empty, error (401/403/404/409/422/429/501/503) and offline states on every screen.
- [ ] Push: register, refresh, rotate, sign-out deactivation, tap-to-route, actionable categories and badge counts verified per CLIENT-ARCHITECTURE §6 (Windows/Linux: local notifications from polling).
- [ ] Checkout and portal round-trips verified in Stripe test mode, including the "Finishing up…" poll.
- [ ] Accessibility checklist passes with the platform screen reader and the largest text size.
- [ ] Performance budgets met (NON-FUNCTIONAL.md). No secrets, tokens or notification content in logs or analytics.
