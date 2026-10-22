# Remaining manual steps

One-time setup that has to be done by hand, in a console or from a laptop, to take the Platform API live. Code that still has to be built is in [plan-remaining.md](plan-remaining.md); how the deployment works is in [infra/README.md](infra/README.md).

Already done (2026-10-20): the AWS account (`substratal-prod`, `196936048600`) and its management account; CLI access with `aws login`; DNS moved from Vercel to Route 53; the Organization SCP exception for `substratal-prod`; the bootstrap Terraform (budgets, anomaly detection, state and artifact buckets, GitHub OIDC roles); GitHub variables, the `production` environment limited to `main`, and the `main` ruleset; and a first partial deploy (Aurora, all five Lambdas, API Gateway, the certificate, SES domain). `https://api.substratalapps.com/health` returns `200` (the API moved from `/v1/…` paths to the `Substratal-Version` header on 2026-10-21; see [Conventions → Versioning](https://compositecode.github.io/substratalapps.com/api-reference/conventions/#versioning)).

Work through these in order. The first four make the API usable; the rest make it ready for real customers.

1. [Raise the Lambda concurrency quota](#1-raise-the-lambda-concurrency-quota)
2. [Re-run the production deploy](#2-re-run-the-production-deploy)
3. [Set the superadmin's password](#3-set-the-superadmins-password)
4. [Confirm the alert emails](#4-confirm-the-alert-emails)
5. [Get SES out of the sandbox](#5-get-ses-out-of-the-sandbox)
6. [Set up Stripe](#6-set-up-stripe)
7. [Add push credentials](#7-add-push-credentials)
8. [Register Substratal's Applications](#8-register-substratals-applications)
9. [Fix the pull-request plan job](#9-fix-the-pull-request-plan-job)
10. [Legal and compliance](#10-legal-and-compliance)
11. [Optional: transfer the domain registration](#11-optional-transfer-the-domain-registration)

---

## 1. Raise the Lambda concurrency quota

New AWS accounts can run only **10** concurrent Lambda executions, and AWS keeps 10 unreserved, so the webhook worker's reserved concurrency of 5 fails the deploy (`decreases account's UnreservedConcurrentExecution below its minimum value of [10]`). It would also cap the whole API at 10 simultaneous requests.

1. Sign in to the **substratal-prod** console session, region **US East (N. Virginia)**.
2. Open https://us-east-1.console.aws.amazon.com/servicequotas/home/services/lambda/quotas/L-B99A9384 (**Service Quotas → AWS services → AWS Lambda → Concurrent executions**).
3. **Request increase at account level**, enter **1000**, and submit.
4. Watch it under **Service Quotas → Quota request history**. It's usually approved within hours, sometimes a couple of days, and AWS emails the account contact.

From the CLI instead: `aws service-quotas request-service-quota-increase --service-code lambda --quota-code L-B99A9384 --desired-value 1000 --profile substratal-prod --region us-east-1`.

## 2. Re-run the production deploy

After the quota shows **1000** (`aws lambda get-account-settings --profile substratal-prod --query AccountLimit.ConcurrentExecutions`):

- **Easiest:** GitHub → **Actions → Deploy → Run workflow**, branch `main`, **Run workflow**.
- Or from a terminal: `gh workflow run deploy.yml -R CompositeCode/substratalapps.com --ref main`.

The run builds the Lambdas, applies Terraform (it picks up where the first run stopped), runs the migrations on the new cluster, makes sure the protected superadmin exists (`SUPERADMIN_EMAIL`), and smoke-tests `/health` and JWKS. It takes about 5–10 minutes. Every later merge into `main` deploys the same way, automatically.

## 3. Set the superadmin's password

The deploy creates `adronhall@proton.me` as the **protected superadmin**: active, verified, holding `superadmin`, and impossible to suspend, delete, erase, or demote ([Users → The protected superadmin](https://compositecode.github.io/substratalapps.com/api-reference/users/#the-protected-superadmin)). It has no password until you set one. From a clone of the repository, signed in with `aws login --profile substratal-prod`:

```bash
export AWS_PROFILE=substratal-prod AWS_REGION=us-east-1
export DATABASE_BACKEND=dataapi \
  DB_CLUSTER_ARN=arn:aws:rds:us-east-1:196936048600:cluster:substratal-prod-db \
  DB_SECRET_ARN=$(aws rds describe-db-clusters --db-cluster-identifier substratal-prod-db \
    --query 'DBClusters[0].MasterUserSecret.SecretArn' --output text)
go run ./cmd/admin bootstrap -email adronhall@proton.me   # prompts for the password (12+ characters)
```

Then sign in: `curl -s https://api.substratalapps.com/auth/login -H 'Substratal-Version: 1' -H 'content-type: application/json' -d '{"email":"adronhall@proton.me","password":"…"}'`. Turn on MFA right away ([Auth → MFA](https://compositecode.github.io/substratalapps.com/api-reference/auth/#mfa-totp)).

## 4. Confirm the alert emails

- **Budget and anomaly alerts:** SNS sent a subscription confirmation ("AWS Notification - Subscription Confirmation") for the `substratal-billing-alerts` topic to `adronhall@proton.me`. Click **Confirm subscription**; until then the $75 warning, the $150 hard stop, and anomaly alerts reach no one.
- **Operational alarms** (5xx rate, Lambda errors and throttles, Aurora near capacity, webhook DLQ, security log): the `alarm_email` Terraform variable is empty, so these alarms have no recipient. Decide the address; setting it is a one-line change to `infra/terraform/prod/variables.tf` and a confirmation click after the next deploy.

## 5. Get SES out of the sandbox

SES is still in the **sandbox**: it can only send to verified addresses, so signup verification, password resets, and invitations don't reach real users.

1. **substratal-prod** console → **Amazon SES → Account dashboard → Request production access**.
2. Mail type **Transactional**; website `https://compositecode.github.io/substratalapps.com/`; describe the use: account verification, password resets, invitations, and security notices for users of Substratal's Applications, sent from `mail.substratalapps.com` (DKIM/SPF/DMARC configured), with bounces and complaints suppressed.
3. AWS usually answers within a day.

Until then, verify any address you need to test with under **SES → Identities → Create identity → Email address**.

## 6. Set up Stripe

The billing code (Free and Standard at $6 per user per month) is built and deployed; Stripe itself isn't set up. See [Pricing](https://compositecode.github.io/substratalapps.com/pricing/) and [infra/README.md → Stripe Billing](infra/README.md#stripe-billing).

1. **Activate the Stripe account:** business details, bank account, and the public statement descriptor. Live mode needs a privacy policy and terms of service URL (step 10).
2. **Enable Stripe Tax** and add your tax registrations (billing is tax-exclusive).
3. **Create the catalog**, test mode first: `STRIPE_SECRET_KEY=sk_test_… make stripe-catalog` creates the `substratal_standard` Product and the `standard_monthly_usd` Price ($6.00 per user, monthly). Repeat with the live key (`CONFIRM_LIVE=yes`) when going live.
4. **Configure the Customer Portal** (Settings → Billing → Customer portal): allow updating payment methods, viewing invoices, and canceling at period end. **Turn plan switching off.**
5. **Register the webhook endpoint** `https://api.substratalapps.com/internal/stripe/webhook` with these events: `checkout.session.completed`, `customer.subscription.created`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.paid`, `invoice.payment_failed`. Copy its signing secret.
6. **Put the keys into the app secret** `substratal-prod/app` (Secrets Manager → `substratal-prod/app` → Retrieve secret value → Edit): set `stripe_secret_key` (a restricted key with Customers, Subscriptions, Checkout Sessions, Customer Portal, and Prices read access) and `stripe_webhook_secret`. Terraform never overwrites these. The Lambdas pick them up on their next cold start; re-running the deploy forces it.
7. **Decide test or live.** Production holds one key. With a test key, the whole flow can be tried end to end with Stripe's test cards; with the live key, test-mode Users can't check out (`409 test_mode_billing_unavailable`, by design). Start with test, then switch to live.

## 7. Add push credentials

Push notifications are merged (PR #12) and ship with the next successful deploy, which also applies migration `0003_push.sql`. Migrations `0003_push` and `0004_protected_superadmin` touch different tables, so it doesn't matter that `0004` reached `main` first.

For each Application that sends pushes, its owner uploads credentials with `POST /applications/{id}/push-credentials` (and rotates them with `PATCH …/push-credentials/{provider}`): an APNs key (`.p8`, key id, team id, bundle id), an FCM service account, or Web Push VAPID keys ([Push Notifications → Push credentials](https://compositecode.github.io/substratalapps.com/api-reference/notifications/#push-credentials)).

## 8. Register Substratal's Applications

For each of Substratal's apps, with the superadmin's token and `Substratal-Version: 1` on every request ([Applications → Owner onboarding](https://compositecode.github.io/substratalapps.com/api-reference/applications/#owner-onboarding-end-to-end)):

1. `POST /applications` with its slug, name, `launch_url`, `redirect_uris`, `permissions`, `available_app_roles`, and **`access_tier`** (`free` or `standard`, which decides who gets it under [Pricing](https://compositecode.github.io/substratalapps.com/pricing/)).
2. Give each seeded Role its permissions (`PATCH /roles/role_<slug>_<name>`).
3. Create the app backend's key (`POST /api-keys`, scope the app, `entitlements.manage` and `users.list`), plus a test-mode key.
4. Subscribe the backend to `access.revoked` and `role.removed` (`POST /webhooks`): the required revocation integration.

## 9. Fix the pull-request plan job

The `plan` job on pull requests into `main` fails: the read-only plan role can't read the app secret's value (`secretsmanager:GetSecretValue`), which `terraform plan` needs to refresh `aws_secretsmanager_secret_version.app`. It isn't a required check, so it doesn't block merges, but its plan comment is the best preview of what a release changes in AWS. Choose one:

- Allow the plan role `secretsmanager:GetSecretValue` on `substratal-prod/app` only (it then can read the Stripe keys, as anyone who can open a PR into `main` could), or
- Stop Terraform reading the value: keep the secret's initial value out of Terraform state and plan with the secret version untracked. This is a small Terraform change.

## 10. Legal and compliance

- **Privacy policy and terms of service**, published at stable URLs. Stripe live mode and SES production access both expect them.
- **Data processing agreements** with the sub-processors listed on [Compliance](https://compositecode.github.io/substratalapps.com/compliance/): AWS (AWS Artifact → Agreements), Stripe, and the push providers (Apple, Google, and the browser push services).

## 11. Optional: transfer the domain registration

DNS is on Route 53; the registration is still at Vercel (reseller for Tucows), renewing 2027-08-07. To consolidate: in Vercel, **Domains → substratalapps.com → Transfer out** to get the authorization code, then `aws route53domains transfer-domain --domain-name substratalapps.com --duration-in-years 1 --auth-code … --region us-east-1` with the registrant contact details (about $15, adds a year). Keep the Route 53 nameservers during the transfer.

Also keep an eye on the Organization SCP exception for `substratal-prod` ([infra/README.md → AWS account & region](infra/README.md#aws-account--region)): if production calls suddenly fail with an explicit SCP deny in `us-east-1`, that exception was removed.
