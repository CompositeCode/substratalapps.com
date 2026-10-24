---
layout: default
title: Account and Data Deletion
parent: Legal
nav_order: 9
permalink: /legal/data-deletion/
---

# Account and Data Deletion
{: .no_toc }

How to delete your Substratal account and personal data, what's removed, and what's kept.
{: .fs-5 .fw-300 }

## Delete your account yourself

**In any Substratal Hub app** (web, macOS, Windows, Linux, iOS, Android): **Account › Privacy › Delete account**, then confirm.

**With the API**, signed in as yourself:

```
POST https://api.substratalapps.com/users/me/erasure-requests
Authorization: Bearer <your access token>
Substratal-Version: 1
```

If you have a Standard subscription, cancel it first in **Plan & Billing › Manage billing**. Deleting your account doesn't stop billing on its own.

## Ask us to delete it

Email `privacy@substratalapps.com` **from the address on your account**, with the subject "Delete my account." We'll confirm it's you and process the request within 30 days, usually much sooner.

## What happens

1. **Immediately:** your account is deactivated. You're signed out on every device, push notifications stop, and applications stop accepting your sign-in.
2. **For 7 days:** you can change your mind. Email `privacy@substratalapps.com` and we'll cancel the deletion.
3. **After 7 days:** your account is permanently erased. That removes your password hash and two-factor secret, your profile, settings, per-application data, organization memberships, devices, and notifications, and removes your personal details from audit records.

## What's kept

- **The fact that actions happened**, without your personal details, in our audit records, to keep the security record intact.
- **Billing records** that tax and accounting law requires us or Stripe to keep, generally for 7 years.
- **Data a developer's Application keeps in its own systems.** Ask that Application's developer to delete it.

## Delete only some data, or get a copy first

- **Export everything we hold about you:** **Account › Privacy › Export** in the Hub, or `GET /users/me/export`.
- **Turn off notifications** on a device, or remove a device: **Account › Devices & notifications**.
- **Remove access to a single application**, or leave an organization, without deleting your account: in the Hub, or ask that application's developer or organization's administrator.

See the [Privacy Policy](../privacy/) for your other rights.
