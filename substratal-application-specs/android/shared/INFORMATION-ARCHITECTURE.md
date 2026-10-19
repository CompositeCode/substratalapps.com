# Information Architecture

One navigation model, expressed natively on each platform. Section names, order, grouping and icon meanings are identical everywhere.

## 1. Navigation tree (screen IDs and routes)

```
Substratal Hub
├── PERSONAL
│   ├── Home                 /                       APP-01  (APP-02 /apps/:appId, APP-05 /apps/past)
│   ├── Inbox  (unread n)    /inbox                  NTF-01  (NTF-02 /inbox/:notificationId)
│   ├── Organizations        /orgs                   ORG-01  (ORG-03 /orgs/:orgId/members, ORG-06 …/apps, BIL-02 …/subscription, ORG-08 …/settings)
│   ├── Account              /account                ACC-01  (ACC-04 /account/security, NTF-03 /account/devices, ACC-07 /account/privacy)
│   ├── Plan & Billing       /billing                BIL-01  (BIL-03 /billing/return)
│   └── Settings             /settings               SET-01  (SET-02 /settings/apps/:appId)
├── ADMINISTRATION           /admin   (GLB-02 Manage hub on compact layouts)
│   ├── Users                /admin/users            USR-01  (USR-03 /admin/users/:userId + tabs Access ACX-01, Roles ROL-04, Sessions USR-04, Notifications NTF-05, Activity)
│   ├── Access               /admin/access           ACX-05
│   ├── Roles                /admin/roles            ROL-01  (ROL-02 /admin/roles/:roleId)
│   ├── Audit Log            /admin/audit            AUD-01  (AUD-02 /admin/audit/:eventId)
│   └── Send notification    /admin/notifications/new NTF-06
└── DEVELOPER                /dev     (GLB-02 Manage hub on compact layouts)
    ├── Applications         /dev/apps               APL-01  (APL-03 /dev/apps/:appId + tabs incl. Push APL-07)
    ├── API Keys             /dev/keys               KEY-01  (KEY-04 /dev/keys/:keyId)
    ├── Webhooks             /dev/webhooks           WHK-01  (WHK-03 /dev/webhooks/:hookId)
    └── Tenant               /dev/tenant             BIL-04

Outside the shell:  /sign-in  /sign-in/mfa  /sign-up  /verify-email  /forgot-password  /reset-password  /invite
Overlays:           Command palette GLB-01 · Turn on notifications NTF-04 · System states GLB-03
```

## 2. Visibility rules (from `Capabilities`)

| Item | Visible when |
|---|---|
| Home, Inbox, Account, Plan & Billing, Settings | always (Inbox, devices: flag `notifications`; Plan & Billing: flag `billing`) |
| Organizations | flag `organizations` |
| → Organization › Subscription tab | `org_admin` of that org, or `billing.manage` |
| Administration section | any of `users.list`, `entitlements.manage`, `roles.manage`, `audit.view`, `notifications.send`, or owns ≥ 1 app (Access only, confined) |
| → Users | `users.list` |
| → Access | `entitlements.manage` or owns ≥ 1 app |
| → Roles | `users.list` (read) / `roles.manage` (write) |
| → Audit Log | `audit.view` |
| → Send notification | platform `notifications.send` |
| Developer section | owns ≥ 1 app or Tenant, or any of `applications.manage`, `api_keys.manage`, `webhooks.manage`, `tenants.manage` |
| → Application › Push tab | owner or `applications.manage` (write needs a User token — always true in the Hub) |
| → Tenant | Tenant owner or `tenants.manage` |

Hide what the person can never use. Show but **disable with a reason** what they can see but not do right now ("Managed by Substratal — you can only turn it off", "You're the last admin", "Seed roles can't be edited").

## 3. Navigation container per platform

| Platform | Wide layout | Compact layout | Primary actions |
|---|---|---|---|
| macOS | `NavigationSplitView`: sidebar (full tree) · list · detail | min window 900 pt — no compact mode | toolbar trailing item |
| Windows | `NavigationView` Left; Account, Plan & Billing, Settings in pane footer | LeftCompact → LeftMinimal | page header button + CommandBar |
| iOS / iPadOS | iPad: sidebar-adaptable `TabView` (full tree) + split views | iPhone: 5 tabs — **Home · Inbox · Orgs · Manage · Account** (Plan & Billing and Settings inside Account) | nav-bar trailing button |
| Android | expanded: permanent drawer (full tree); medium: rail | compact: bottom bar, same 5 destinations | Extended FAB |
| Linux (GNOME) | `AdwNavigationSplitView` sidebar (full tree) | single pane < 600 sp | header-bar suggested button |
| Web | ≥ 1200 px sidebar + resizable panes; 768–1199 collapsible sidebar + 2 panes | < 768: bottom nav (same 5) + sheets | toolbar button · floating primary button (phone) |

Rule: **wide = every section in a sidebar; compact = 5 destinations with Administration and Developer combined in the Manage hub (GLB-02), which is removed when the person has no rights in either.** The Inbox destination carries the unread badge on every layout.

## 4. Routes and deep links

The route column in `SCREEN-SPECS.md` is canonical. Web host: **`https://dashboard.substratalapps.com`** (the host the API spec uses in its billing examples).

| Platform | Form |
|---|---|
| Web | `https://dashboard.substratalapps.com{route}` |
| iOS, macOS | Universal Links on the same URLs + `substratal://{route}` |
| Android | Verified App Links on the same URLs + `substratal://{route}` |
| Windows | App URI handler on the same URLs + `substratal:{route}` |
| Linux | `x-scheme-handler/substratal` → `substratal://{route}` |

Routes encode the view's state (tab, filters as query). Emailed links (`/verify-email?token=`, `/reset-password?token=`, `/invite?token=`, `/orgs/:orgId/invitation`) and Stripe returns (`/billing/return?checkout=…`, `/orgs/:orgId/subscription?checkout=…`) open the installed app when present, else the web app. A notification's `action_url` that is a Hub route opens in-app; any other URL opens externally. Restoring a route without permission shows the Forbidden state, not a redirect.

## 5. Section icons (meaning → platform symbol)

| Section | SF Symbols | Fluent | Material Symbols | Adwaita | Web (Lucide) |
|---|---|---|---|---|---|
| Home | `square.grid.2x2` | `Apps` | `apps` | `view-app-grid-symbolic` | `layout-grid` |
| Inbox | `tray` (badge) | `MailInbox` | `inbox` | `mail-inbox-symbolic` | `inbox` |
| Organizations | `person.3` | `PeopleTeam` | `groups` | `system-users-symbolic` | `users` |
| Account | `person.crop.circle` | `PersonCircle` | `account_circle` | `avatar-default-symbolic` | `circle-user` |
| Plan & Billing | `creditcard` | `Payment` | `credit_card` | `wallet-symbolic` | `credit-card` |
| Settings | `gearshape` | `Settings` | `settings` | `emblem-system-symbolic` | `settings` |
| Manage (compact) | `slider.horizontal.3` | `Options` | `tune` | `preferences-system-symbolic` | `sliders-horizontal` |
| Users | `person.2` | `PeopleList` | `group` | `contact-new-symbolic` | `contact` |
| Access | `switch.2` | `ToggleRight` | `toggle_on` | `object-select-symbolic` | `toggle-right` |
| Roles | `checkmark.shield` | `ShieldKeyhole` | `admin_panel_settings` | `security-high-symbolic` | `shield-check` |
| Audit Log | `list.bullet.clipboard` | `History` | `history` | `document-open-recent-symbolic` | `history` |
| Send notification | `bell.badge` | `AlertUrgent` | `notification_add` | `bell-symbolic` | `bell-plus` |
| Applications | `shippingbox` | `Box` | `widgets` | `application-x-addon-symbolic` | `box` |
| API Keys | `key` | `Key` | `key` | `dialog-password-symbolic` | `key-round` |
| Webhooks | `arrow.triangle.branch` | `PlugConnected` | `webhook` | `network-transmit-receive-symbolic` | `webhook` |
| Tenant | `server.rack` | `Server` | `dns` | `network-server-symbolic` | `server` |

## 6. Standard screen anatomy

| Archetype | Wide | Compact |
|---|---|---|
| list / table | list pane (table on desktop) with search + filter chips | full-width rows, search in nav bar, filters in chip row or sheet |
| detail | header (title, ID chip, status) + tabs + content; actions in toolbar | header + segmented control; actions in nav bar / overflow |
| form / wizard | sheet / dialog | full-screen sheet / dialog |
| confirm | alert/dialog naming the object; destructive button styled destructive | same |
| secret | non-dismissible modal | same |
| popover | anchored popover or inspector pane | half-height sheet |
| handoff | leaves the app; returns via deep link | same |
| hub | (not used — sidebar replaces it) | grouped list |
