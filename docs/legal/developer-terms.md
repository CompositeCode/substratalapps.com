---
layout: default
title: Developer and API Terms
parent: Legal
nav_order: 4
permalink: /legal/developer-terms/
---

# Developer and API Terms
{: .no_toc }

**Effective October 23, 2026.** These terms apply, in addition to the [Terms of Service](../terms/), when you register an Application on Substratal, use an API Key, call the Substratal API or MCP server, or receive webhooks. "**You**" is the developer, and the organization you act for.
{: .fs-5 .fw-300 }

1. TOC
{:toc}

---

## 1. Access to the API

We grant you a non-exclusive, non-transferable, revocable license to use the Substratal API, its documentation, and `openapi.yaml` to build and run Applications that use Substratal for user accounts, access, settings, organizations, storage, and notifications, under these terms and the [documentation](../../).

## 2. Registering an Application

- Give accurate information about your Application, keep it current, and keep its contact and support details working.
- A self-registered Application starts in review and can't go live until we approve it. We may reject or suspend an Application that breaks these terms, the [Acceptable Use Policy](../acceptable-use/), or the review policy in [Applications → The review lifecycle](../../domain-model/applications/#the-review-lifecycle).
- A self-registered Application must use the hosted sign-in flow. It may never ask users for their Substratal password.

## 3. Credentials

- Keep API Keys, webhook signing secrets, and push credentials confidential, and use API Keys only from your servers, **never from a browser or a distributed app**. The API refuses an API Key sent from a browser.
- Give keys the least permission they need. Rotate or revoke a key promptly if it may have been exposed, and tell us at `security@substratalapps.com` if user data may have been affected.
- You're responsible for all activity under your Application's credentials.

## 4. Your users and their data

- **Your own privacy policy.** Publish a privacy policy for your Application that accurately describes what you do with your users' data, and get any consent the law requires. Substratal doesn't run your signup form or collect consent for you.
- **Use data only to run your Application.** Use the user data you receive from Substratal only to provide your Application's features to the user it belongs to. Don't sell it, use it for unrelated advertising, or combine it to identify people across services without their consent.
- **Honor access changes.** When a user's access is revoked (the `access.revoked` and `role.removed` webhooks), stop serving them promptly. Don't cache access decisions longer than the token or webhook says they're valid.
- **Processor terms.** For your users' personal data, you're the controller and we're your processor under the [Data Processing Addendum](../dpa/), which forms part of these terms.
- **Push content.** Keep sensitive personal information out of push notification text; push payloads pass through Apple, Google, and browser push services.

## 5. Limits and fair use

Respect the documented [rate limits](../../non-functional-requirements/#rate-limiting) and don't try to get around them, including by using multiple accounts or keys. Don't use the API to build a product that substitutes for Substratal itself, or to scrape data you aren't authorized to access. We may set or change limits to protect the Service, with notice where practical.

## 6. Changes to the API

The API is versioned with the `Substratal-Version` header. Additive changes ship within a version. A breaking change ships as a new version, and the previous version keeps working for **at least 6 months**, announced in the [Changelog](../../changelog/), with `Deprecation` and `Sunset` headers on its responses. See [Conventions → Versioning](../../api-reference/conventions/#versioning).

## 7. Branding

You may say your Application "uses Substratal" or offers "Sign in with Substratal," accurately and without implying endorsement. Don't use the Substratal name or logo in your Application's name or as its primary branding without written permission. See [Copyright, Licenses and Trademarks](../copyright/).

## 8. Fees

The API is free for developers. Your users pay for Substratal plans as described in [Pricing](../../pricing/). If you bill your own users, you do that outside Substratal, under your own terms.

## 9. Suspension and termination

We may suspend or revoke your credentials or Application to protect users or the Service, or if you breach these terms, telling you why where practical. You may stop using the API at any time. When these terms end, stop using the API, delete user data you received from it unless the law requires you to keep it or the user has a separate relationship with you, and remove Substratal branding.

## 10. Warranties, liability and law

The disclaimers, limitation of liability, indemnity, and governing law (Washington State, USA; courts of King County) in the [Terms of Service](../terms/) apply. You'll indemnify Composite Code against claims arising from your Application or your breach of these terms.

## 11. Contact

Composite Code LLC · `legal@substratalapps.com` · security: `security@substratalapps.com`
