---
layout: default
title: Sub-processors
parent: Legal
nav_order: 7
permalink: /legal/subprocessors/
---

# Sub-processors
{: .no_toc }

**Updated October 23, 2026.** The third parties that process personal data to run Substratal. We announce a new sub-processor here and in the [Changelog](../../changelog/) at least 30 days before it starts processing personal data (see the [Data Processing Addendum](../dpa/)).
{: .fs-5 .fw-300 }

| Sub-processor | What they do | Data | Location |
|---|---|---|---|
| **Amazon Web Services, Inc.** | Hosting: compute (Lambda), database (Aurora PostgreSQL), storage (S3), keys (KMS), queues, logging, and transactional email (Amazon SES), including mail sent to our contact addresses | All Service data | United States (`us-east-1`) |
| **Stripe, Inc.** | Subscription payments, invoices, tax calculation, and the billing portal | Name, email, billing address, payment details (held by Stripe), subscription status | United States and Stripe's global processing locations |
| **Apple Inc.** | Push notifications to iOS, iPadOS and macOS devices (APNs), and Safari web push | Push token and notification content | United States and Apple's global locations |
| **Google LLC** | Push notifications to Android devices (Firebase Cloud Messaging) and Chrome web push | Push token and notification content | United States and Google's global locations |
| **Mozilla Corporation** | Firefox web push | Push token and notification content | United States |
| **Microsoft Corporation** | Edge web push | Push token and notification content | United States and Microsoft's global locations |

Push services receive only what's needed to deliver a notification to a device you turned notifications on for. Windows and Linux Hub apps don't use a push service; they check for new notifications themselves.

**Not sub-processors:** GitHub, Inc. hosts this documentation site and our source code, but doesn't process Service user data. Applications built by other developers are their own controllers.
