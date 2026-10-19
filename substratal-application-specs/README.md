# Substratal Applications Specifications

Build specifications for the six **Substratal Hub** user-interface apps — **macOS, Windows, iOS/iPadOS, Android, Linux (GNOME) and Web (responsive PWA, based on the macOS design)** — against the **Substratal Apps Platform API spec v1.4.0** (2026-10-18; `openapi.yaml` 1.0.0-draft.3, 118 operations): https://compositecode.github.io/substratalapps.com/

This edition adds the API's **push notifications and notification inbox**, the **Free / Standard** subscription model, and **platform-managed entitlements**. See `00-shared/CHANGES.md` for everything that moved. This edition also adds the **Substratal logo standard** (`00-shared/brand/`) with a complete logo and app-icon kit for every platform (`<platform>/brand-assets/`). The logo is placed in every design. It also adds the **theme system**: 5 light and 5 dark themes, defaults **Moss Paper** (light) and **Forest Floor** (dark), user-selectable in Settings › Appearance (`00-shared/THEMES.md`). Every screen is rendered in both defaults, and each platform has its own theme code in `<platform>/theme/`.

## Layout

```
substratal-application-specs/
├── README.md
├── 00-shared/                       source of truth for every team
│   ├── THEMES.md                    theme system: roles, 10 palettes, contrast, persistence, platform mapping
│   ├── themes/                      themes.json · themes.csv · themes-overview.png
│   ├── brand/                       logo standard: BRAND.md, tokens, SVG masters, PNG exports, logo-standard.png
│   ├── CHANGES.md                   what changed since the previous package (API v1.2.2 → v1.4.0)
│   ├── PRODUCT-SPEC.md              scope, personas, 13 domains → 75 screens, phases, acceptance criteria
│   ├── SCREEN-SPECS.md              every screen: route, who, loads, elements (kind · data · rules), actions (API call · result handling), states
│   ├── INFORMATION-ARCHITECTURE.md  nav tree + routes, visibility rules, per-platform containers, deep links, icons
│   ├── CLIENT-ARCHITECTURE.md       sessions, request conventions, Capabilities, the Hub as app_hub, push + inbox, caching, billing hand-off, flags, security
│   ├── UX-FLOWS.md · flows/*.svg · ux-flows.html   16 end-to-end flows
│   ├── COMPONENTS.md                behavior contracts + native mapping on all 6 platforms
│   ├── COPY-AND-STATES.md           labels, confirmations, empty states, upsell copy, error messages, notification actions
│   ├── ACCESSIBILITY.md · NON-FUNCTIONAL.md · OPEN-QUESTIONS.md
│   ├── status-semantics.json        status → label, semantic role, icons per platform
│   └── API-SCREEN-MAP.csv           all 118 operations → screens (100 % covered)
└── macos/ windows/ ios/ android/ linux/ web/      one self-contained folder per team
    ├── README.md                    reading order + definition of done
    ├── PLATFORM-SPEC.md             stack, navigation, interaction, push/notifications, billing, integrations, distribution, testing
    ├── SCREENS.md                   all 75 screens with this platform's container and native control per element
    ├── theme/                       native theme code (Swift / XAML+C# / Kotlin / GTK CSS / CSS+TS)
    ├── brand-assets/                app icons + logo files in this platform's native formats
    ├── screens/{light,dark}/*.png · mockups/     reference renders (web: desktop, tablet, phone + interaction examples)
    └── shared/                      copy of 00-shared
```

## Push notifications at a glance

| App | How notifications arrive |
|---|---|
| iOS, iPadOS, macOS | APNs, as first-party app `app_hub` (one bundle id across iOS and macOS) |
| Android | FCM, as `app_hub` |
| Web | Web Push (VAPID), as `app_hub`; installed PWA on iOS/iPadOS |
| Windows, Linux | The API has no push provider for them — the app polls and shows local notifications |

Every app shows the same unified **Inbox** of all apps' notifications plus Substratal's security and account alerts.
