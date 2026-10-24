---
layout: default
title: Privacy Policy
parent: Legal
nav_order: 2
permalink: /legal/privacy/
---

# Privacy Policy
{: .no_toc }

**Effective October 23, 2026.** How **Composite Code LLC** ("we", "us"), the operator of Substratal Apps, collects, uses, shares, and protects personal information, and the choices and rights you have.
{: .fs-5 .fw-300 }

1. TOC
{:toc}

---

## 1. Who we are and what this covers

Composite Code LLC, a Washington limited liability company, is the **controller** of personal information processed to run Substratal: the platform API at `api.substratalapps.com`, the Substratal Hub apps (web at `dashboard.substratalapps.com`, macOS, Windows, Linux, iOS and Android), Substratal's own applications, and this site.

When a **developer's own Application** or an **Organization** uses Substratal to manage its users, that developer or Organization decides why and how its users' data is used, and we process that data on its behalf as a **processor** under the [Data Processing Addendum](../dpa/). That Application's or Organization's own privacy policy then applies too.

Contact: `privacy@substratalapps.com`.

## 2. What we collect

**You give us:**

| Category | Examples |
|---|---|
| Account | Email address, password (stored only as an Argon2id hash, never in readable form), display name, avatar URL, locale and time zone |
| Security | Two-factor (TOTP) secret, stored encrypted; recovery codes, stored hashed |
| Preferences and app data | Settings, per-application profiles and settings an application stores for you |
| Organizations | Organizations you create or join, your role in them, and invitations |
| Billing | Your plan and subscription status. **Card details go directly to Stripe**; we store only Stripe's customer and subscription references, never card numbers. |
| Communications | Messages you send to our contact addresses |

**Created when you use Substratal:**

| Category | Examples |
|---|---|
| Sessions | When each session started and was last used, its IP address and user agent, and how you signed in (password, two-factor) |
| Devices | Push tokens and a name for each device you turn notifications on for |
| Notifications | Notifications sent to you, and whether they were delivered and read |
| Access | Which applications you can use, and your roles and permissions in them |
| Audit records | A record of security-relevant and administrative actions, such as sign-ins, changes to access, and who made them |
| Logs | Request and security logs (request ID, operation, status, IP address, user agent), kept to run and protect the Service |

**We don't** collect precise location, contacts, or advertising identifiers, and we don't use third-party advertising or analytics trackers.

## 3. How we use it, and our legal bases

| Purpose | Legal basis (EEA/UK) |
|---|---|
| Create and run your account, sign you in, and provide the apps and features you use | Performance of our contract with you |
| Process subscription payments and keep billing records | Contract; legal obligation (tax and accounting) |
| Send transactional email: verification, password resets, invitations, and security notices such as a new sign-in | Contract; legitimate interest in account security |
| Send push notifications and keep your inbox, if you turn them on | Contract; consent for push, which you can turn off any time |
| Protect the Service: detect abuse, enforce rate limits, investigate security incidents, keep audit records | Legitimate interests in security and integrity; legal obligation |
| Troubleshoot and improve the Service | Legitimate interests |
| Comply with law and respond to lawful requests | Legal obligation |

We don't sell personal information, don't "share" it for cross-context behavioral advertising, and don't use it to train AI models. We don't make decisions about you based solely on automated processing that produce legal or similarly significant effects.

## 4. Who we share it with

- **Service providers** that host and run Substratal for us, under contracts that limit their use of the data: Amazon Web Services (hosting, database, email delivery through Amazon SES), Stripe (payments), and Apple, Google, Mozilla and Microsoft push services (delivering notifications to your devices). See the current list on [Sub-processors](../subprocessors/).
- **Applications you use.** When you open an application, it receives an access token with your user ID, email, email-verification status, whether you have access, and your permissions in that application. An application's developer can see the information you give that application.
- **Your Organization.** Its administrators can see your name, email, role, and membership, and manage your access to its applications.
- **Legal and safety.** When required by law, or to protect the rights, safety, or property of our users, the public, or Composite Code.
- **Business transfers.** If Composite Code is involved in a merger, acquisition, or sale of assets, subject to this policy.

## 5. How long we keep it

| Data | Kept |
|---|---|
| Account and profile | Until you delete your account. Deletion takes effect immediately and the account is **permanently erased 7 days later** ([Account and Data Deletion](../data-deletion/)). |
| Sessions | Until they end: 30 days after last use, 90 days at most |
| Notifications | 90 days. Delivery details: 30 days. Inactive devices: deleted 30 days after they stop receiving notifications |
| Audit records | Full detail for one year. After that, and immediately when an account is erased, the personal details are removed and only the record that an action happened remains |
| Billing records | As long as tax and accounting law requires (generally 7 years), held by Stripe and us |
| Stored copies of mail sent to our contact addresses | 30 days; the forwarded copy is kept as long as needed to handle your request |
| Test-mode accounts created by developers | 30 days at most |

## 6. Your rights and choices

Depending on where you live (including under the GDPR, UK GDPR, and California's CCPA/CPRA), you may have the right to:

- **access** your personal information and **receive a copy** in a portable format: Hub › Account › Privacy › Export, or `GET /users/me/export`;
- **correct** it: edit your profile in the Hub, or ask us;
- **delete** it: Hub › Account › Privacy › Delete account, or see [Account and Data Deletion](../data-deletion/);
- **object to** or **restrict** certain processing, and **withdraw consent** (for example, turn off push notifications) at any time;
- **not be discriminated against** for exercising these rights.

To exercise a right we don't offer in the Hub, email `privacy@substratalapps.com` from your account's address. We'll verify your identity and respond within 30 days (45 days under the CCPA, extendable where the law allows). You may use an authorized agent; we'll ask for proof of authorization. If your data is held for an Organization or a developer's Application, we may refer your request to them and help them answer it.

You can also complain to your local data-protection authority. We'd appreciate the chance to address your concern first.

## 7. Security

We protect your information with encryption in transit (TLS) and at rest, hashed passwords (Argon2id), encrypted two-factor secrets, optional two-factor authentication, signed short-lived access tokens, least-privilege access for our systems, and audit logging. No system is perfectly secure; if a breach affects your personal information, we'll notify you and the relevant authorities as the law requires, including within 72 hours of becoming aware of it where the GDPR applies. Report vulnerabilities as described in [Security and Vulnerability Disclosure](../security/).

## 8. International transfers

We're based in the United States, and Substratal is hosted in the United States (AWS `us-east-1`). If you're outside the US, your information is transferred to and processed in the US. Where the GDPR or UK GDPR applies, we rely on the European Commission's Standard Contractual Clauses (and the UK Addendum) or another lawful transfer mechanism, with our service providers as well.

## 9. Children

Substratal isn't directed to children under 13 (or the higher age required in your country), and we don't knowingly collect their information. If you believe a child has given us personal information, contact `privacy@substratalapps.com` and we'll delete it.

## 10. Cookies

The web Hub uses only strictly necessary cookies and local storage, to keep you signed in and remember display preferences. See the [Cookie Notice](../cookies/).

## 11. Changes

We'll post updates here with a new effective date, and for material changes notify account holders by email or in the Hub at least 30 days before they take effect.

## 12. Contact

Composite Code LLC · `privacy@substratalapps.com`
