---
layout: default
title: Security and Vulnerability Disclosure
parent: Legal
nav_order: 10
permalink: /legal/security/
---

# Security and Vulnerability Disclosure
{: .no_toc }

We welcome reports from security researchers. If you believe you've found a vulnerability in Substratal, please tell us so we can fix it.
{: .fs-5 .fw-300 }

## How to report

Email **`security@substratalapps.com`** with:

- what you found and where (URL, API operation, or app and version);
- steps to reproduce it, and the impact you believe it has;
- any proof-of-concept, and the request IDs (`X-Request-Id`) of requests you made.

We'll acknowledge your report within **3 business days**, keep you updated, and tell you when it's fixed.

## In scope

- `api.substratalapps.com`, including its MCP server
- `dashboard.substratalapps.com` and the Substratal Hub apps
- Source code in the `CompositeCode/substratalapps*` repositories

## Safe harbor

If you act in good faith under this policy, we won't pursue legal action against you for your research, and we'll consider it authorized under the anti-hacking laws that apply. Please:

- test only against accounts you own, or with the account holder's explicit permission, and prefer test-mode credentials (`satk_test_…`);
- don't access, change, or keep other people's data. If you encounter it, stop, and tell us;
- don't degrade the Service: no denial-of-service, load testing, or automated scanning at volume;
- no social engineering, phishing, or physical attacks on our staff or providers;
- give us a reasonable time to fix the issue (we aim for 90 days) before disclosing it publicly.

## Out of scope

Reports without a demonstrated security impact, such as missing security headers with no exploit, clickjacking on pages with no sensitive actions, self-XSS, rate limits that work as documented, or vulnerabilities in third-party services we don't control (report those to the vendor).

We don't currently run a paid bug bounty, but we're glad to credit researchers who help us, with their permission.
