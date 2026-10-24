---
layout: default
title: Data Processing Addendum
parent: Legal
nav_order: 6
permalink: /legal/dpa/
---

# Data Processing Addendum
{: .no_toc }

**Effective October 23, 2026.** This Data Processing Addendum ("DPA") forms part of the agreement between **Composite Code LLC** ("Processor", "we") and a developer or Organization that uses Substratal to manage its own users ("Customer"): the [Terms of Service](../terms/) and, for developers, the [Developer and API Terms](../developer-terms/). It applies automatically when we process Customer Personal Data. To receive a countersigned copy, write to `legal@substratalapps.com`.
{: .fs-5 .fw-300 }

1. TOC
{:toc}

---

## 1. Definitions

"**Data Protection Law**" means all laws that apply to processing Customer Personal Data under the agreement, including the EU GDPR, the UK GDPR and Data Protection Act 2018, Swiss data-protection law, and US state privacy laws such as the CCPA/CPRA. "**Customer Personal Data**" means personal data that we process on Customer's behalf: the data of Customer's end users and members held in Customer's Applications or Organizations. "Controller," "processor," "data subject," "personal data breach," "processing," "service provider," and "supervisory authority" have the meanings given in Data Protection Law.

## 2. Roles and instructions

Customer is the controller (or a processor for its own customers), and we are the processor (and a "service provider" or "processor" under US state laws). We process Customer Personal Data only on Customer's documented instructions, which are the agreement, Customer's configuration and use of the Service, and API calls Customer makes, unless the law requires otherwise, in which case we'll tell Customer first unless the law forbids it. We'll tell Customer if we believe an instruction violates Data Protection Law.

## 3. Details of processing

| | |
|---|---|
| **Subject matter and duration** | Providing the Service, for the term of the agreement and the deletion period below |
| **Nature and purpose** | Hosting, storing, authenticating, authorizing, transmitting, and deleting data to provide user accounts, access control, settings, organizations, storage, notifications, and audit records |
| **Data subjects** | Customer's end users, members, and administrators |
| **Categories of data** | Identifiers and contact details (name, email), credentials (password hashes, encrypted second-factor secrets), session and device data (IP address, user agent, push tokens), access and role assignments, settings and application data Customer chooses to store, notifications, and audit records |
| **Special categories** | None intended. Customer must not store special-category or protected health data in the Service unless we have agreed to it in writing. |

## 4. Our obligations

- **Confidentiality.** People authorized to process Customer Personal Data are bound by confidentiality.
- **Security.** We implement the technical and organizational measures in Annex 1 and keep them at least as protective as they are today.
- **Sub-processors.** Customer authorizes the sub-processors on [Sub-processors](../subprocessors/). We impose data-protection terms on each that are at least as protective as this DPA, and remain responsible for them. We'll announce a new sub-processor on that page and in the [Changelog](../../changelog/) at least **30 days** before it starts processing Customer Personal Data; Customer may object on reasonable data-protection grounds within that period, and if we can't address the objection, Customer may terminate the affected Service.
- **Data subject requests.** We provide self-service export (`GET /users/{id}/export`) and erasure (`POST /users/{id}/erasure-requests`) and will otherwise reasonably help Customer respond. If we receive a request directly, we'll refer it to Customer.
- **Assistance.** We'll reasonably help Customer with data-protection impact assessments and consultations with supervisory authorities, as they relate to the Service.
- **Breach notification.** We'll notify Customer without undue delay, and in any case within **48 hours**, after becoming aware of a personal data breach affecting Customer Personal Data, with the information Customer needs to meet its own obligations, including the GDPR's 72-hour notification.
- **US state law.** We won't sell or share Customer Personal Data, retain, use, or disclose it outside our direct business relationship with Customer or for any purpose other than providing the Service, or combine it with personal data from other sources except as Data Protection Law permits.

## 5. Deletion and return

Customer can export and delete Customer Personal Data through the API during the term. When the agreement ends, we delete Customer Personal Data within 30 days, and remove it from backups on their normal rotation, except where the law requires us to keep it. Audit records keep only the fact that an action happened once their personal details are removed (see [Data retention](../../non-functional-requirements/#data-retention)).

## 6. Audits

We'll make available the information reasonably necessary to demonstrate compliance with this DPA, including written answers to security questionnaires and, when available, third-party audit reports. If that isn't enough to meet a regulator's requirement, Customer may audit our compliance once a year, with 30 days' notice, at its own cost, under confidentiality, and without disrupting the Service.

## 7. International transfers

Customer Personal Data is processed in the United States. For transfers from the EEA, the **EU Standard Contractual Clauses** (Commission Implementing Decision (EU) 2021/914), Module Two (controller to processor) or Module Three (processor to processor) as applicable, are incorporated by reference, with Clause 7 included, option 2 of Clause 9 with the notice period in section 4, no optional language in Clause 11, governing law and courts of Ireland in Clauses 17 and 18, and Annexes completed by section 3 and Annex 1 of this DPA. For the UK, the **UK International Data Transfer Addendum** applies; for Switzerland, the SCCs apply with the Swiss FDPIC as the competent authority. If the SCCs conflict with this DPA, the SCCs prevail.

## 8. Liability and order of precedence

Each party's liability under this DPA is subject to the limitations in the agreement, to the extent Data Protection Law allows. If this DPA conflicts with the rest of the agreement on processing Customer Personal Data, this DPA prevails.

## Annex 1: Technical and organizational measures

- **Encryption:** TLS for data in transit; encryption at rest for the database, backups, and storage; application secrets and second-factor secrets encrypted with keys in AWS KMS; passwords hashed with Argon2id.
- **Access control:** short-lived signed access tokens; role- and permission-based authorization on every request; least-privilege IAM for every component; production access limited to authorized personnel with multi-factor authentication.
- **Isolation:** test and live data separated by database row-level security; per-Application and per-Organization scoping enforced in the data-access layer.
- **Logging and monitoring:** audit records of administrative and security-relevant actions; security event logging and alarms.
- **Resilience:** managed infrastructure; database storage replicated across availability zones, with automated daily backups kept 7 days and deletion protection.
- **Development:** changes are reviewed and tested in CI before deployment, and infrastructure is defined as code.
- **Incident response:** a documented breach-notification commitment, with audit records as the forensic source.
- **Deletion:** self-service export and erasure, with erasure completed 7 days after a request.
