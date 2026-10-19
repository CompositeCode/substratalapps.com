# Substratal Hub for Windows — Team Handoff

**Standard:** Fluent 2 — Windows 11 · **Stack:** WinUI 3 / Windows App SDK, C#
**API:** Substratal Apps Platform API v1.4.0 (openapi 1.0.0-draft.3) — https://compositecode.github.io/substratalapps.com/

## Reading order

1. `shared/CHANGES.md` — what's new in this edition (push notifications, Free/Standard billing)
2. `shared/PRODUCT-SPEC.md` — scope, personas, domains, phases, acceptance criteria
3. `PLATFORM-SPEC.md` — how this app is built on Windows
4. `SCREENS.md` — all 75 screens with native containers and controls (the build checklist)
5. `shared/CLIENT-ARCHITECTURE.md` — sessions, request conventions, Capabilities, push and inbox (§6), billing hand-off (§9)
6. `shared/UX-FLOWS.md` (+ `shared/flows/*.svg`) — the 16 end-to-end flows
7. `mockups/index.html` / `screens/` — reference renders in the default light (Moss Paper) and dark (Forest Floor) themes; `?mode=dark&light=<id>&dark=<id>` previews any theme
8. While building: `shared/COMPONENTS.md`, `shared/COPY-AND-STATES.md`, `shared/ACCESSIBILITY.md`, `shared/NON-FUNCTIONAL.md`, `shared/OPEN-QUESTIONS.md`, `shared/API-SCREEN-MAP.csv`

**Themes:** `theme/` has this platform's implementation of the 5 light and 5 dark themes. Rules are in `shared/THEMES.md`; wiring is in `PLATFORM-SPEC.md` (Theming section).

**Logo & app icon:** `brand-assets/` has every icon and logo file this app needs. Wiring is in `PLATFORM-SPEC.md` (Brand & app icon section) and rules in `shared/brand/BRAND.md`.

## Definition of done (per screen)

- [ ] Elements, data bindings and API calls match `SCREENS.md`
- [ ] Reachable by its route / deep link; restores state from the route
- [ ] Gated by Capabilities per `shared/INFORMATION-ARCHITECTURE.md` §2
- [ ] Loading, empty, error (401/403/404/409/422/429/503) and offline states
- [ ] Idempotency-Key on creates, If-Match on edits, conflict handling
- [ ] Every gesture/shortcut has a visible equivalent; screen reader and largest text size pass
- [ ] Copy matches `shared/COPY-AND-STATES.md`
- [ ] Correct in both default themes and in every other theme (colours only from theme roles, no literals); high-contrast modes fall back to system colours
- [ ] Logo and icons come only from `brand-assets/` (no redraws), with accessible name "Substratal Apps"
- [ ] Flag-gated features (notifications, push, billing) hide cleanly on `501`

## Reference screens

- `ACC-04-security`: [light](screens/light/ACC-04-security.png) · [dark](screens/dark/ACC-04-security.png)
- `ACX-01-03-access-turn-off`: [light](screens/light/ACX-01-03-access-turn-off.png) · [dark](screens/dark/ACX-01-03-access-turn-off.png)
- `APL-03-application`: [light](screens/light/APL-03-application.png) · [dark](screens/dark/APL-03-application.png)
- `APL-07-push-credentials`: [light](screens/light/APL-07-push-credentials.png) · [dark](screens/dark/APL-07-push-credentials.png)
- `APP-01-home`: [light](screens/light/APP-01-home.png) · [dark](screens/dark/APP-01-home.png)
- `AUD-01-02-audit`: [light](screens/light/AUD-01-02-audit.png) · [dark](screens/dark/AUD-01-02-audit.png)
- `AUTH-01-02-sign-in`: [light](screens/light/AUTH-01-02-sign-in.png) · [dark](screens/dark/AUTH-01-02-sign-in.png)
- `BIL-01-my-plan`: [light](screens/light/BIL-01-my-plan.png) · [dark](screens/dark/BIL-01-my-plan.png)
- `KEY-01-03-api-keys-secret`: [light](screens/light/KEY-01-03-api-keys-secret.png) · [dark](screens/dark/KEY-01-03-api-keys-secret.png)
- `NTF-01-02-inbox`: [light](screens/light/NTF-01-02-inbox.png) · [dark](screens/dark/NTF-01-02-inbox.png)
- `NTF-03-devices`: [light](screens/light/NTF-03-devices.png) · [dark](screens/dark/NTF-03-devices.png)
- `ORG-06-org-apps-seats`: [light](screens/light/ORG-06-org-apps-seats.png) · [dark](screens/dark/ORG-06-org-apps-seats.png)
- `ROL-02-role`: [light](screens/light/ROL-02-role.png) · [dark](screens/dark/ROL-02-role.png)
- `SET-01-appearance-themes`: [light](screens/light/SET-01-appearance-themes.png) · [dark](screens/dark/SET-01-appearance-themes.png)
- `SET-02-app-settings`: [light](screens/light/SET-02-app-settings.png) · [dark](screens/dark/SET-02-app-settings.png)
- `USR-01-users`: [light](screens/light/USR-01-users.png) · [dark](screens/dark/USR-01-users.png)
- `WHK-03-deliveries`: [light](screens/light/WHK-03-deliveries.png) · [dark](screens/dark/WHK-03-deliveries.png)
