-- 0002_subscriptions: subscriber billing (DECISIONS.md #39).
--
-- Pricing bills the people who use Substratal's apps (Free and Standard,
-- individually or through an Organization) instead of the Application
-- owner's Tenant. This adds the subscriptions table, the access tier on
-- Applications, and the platform-managed Entitlement sources, and retires
-- Tenant billing.
--
-- It also contracts in the same file: the Tenant billing columns are
-- dropped here rather than in a later release, because no release has
-- ever run against them (production was never deployed with 0001 alone).

-- ── Subscribers' Stripe Customers ─────────────────────────────────────

alter table users add column stripe_customer_id text unique;
alter table organizations add column stripe_customer_id text unique;

-- ── Subscriptions ─────────────────────────────────────────────────────

create table subscriptions (
  id text primary key,
  user_id text references users(id),
  organization_id text references organizations(id),
  plan text not null default 'standard' check (plan in ('standard')),
  status text not null default 'incomplete'
    check (status in ('active','trialing','past_due','canceled','unpaid','incomplete','incomplete_expired','paused')),
  stripe_subscription_id text unique,
  current_period_start timestamptz,
  current_period_end timestamptz,
  cancel_at_period_end boolean not null default false,
  member_scope text check (member_scope in ('all_members','allowlist','denylist')),
  member_overrides text[] check (cardinality(member_overrides) <= 1000),
  quantity_synced integer,
  quantity_synced_at timestamptz,
  test_mode boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  version integer not null default 1,
  check ((user_id is null) <> (organization_id is null)),
  check (organization_id is not null or (member_scope is null and member_overrides is null)),
  check (organization_id is null or member_scope is not null)
);
-- One subscription row per subscriber, updated in place across lapses and
-- resubscribes; Stripe's own subscription id may change underneath it.
create unique index subscriptions_user_key on subscriptions (user_id) where user_id is not null;
create unique index subscriptions_org_key on subscriptions (organization_id) where organization_id is not null;
create index subscriptions_status on subscriptions (status);

create trigger subscriptions_touch_updated_at before update on subscriptions
  for each row execute function substratal_util.touch_updated_at();
create trigger subscriptions_bump_version before update on subscriptions
  for each row execute function substratal_util.bump_version();

alter table subscriptions enable row level security;
alter table subscriptions force row level security;
create policy subscriptions_mode on subscriptions
  using (substratal_util.mode_visible(test_mode)) with check (substratal_util.mode_visible(test_mode));

-- ── Applications: which tier unlocks them ─────────────────────────────

alter table applications add column access_tier text not null default 'standard'
  check (access_tier in ('free','standard'));
create index applications_access_tier on applications (access_tier);

-- ── Entitlements: platform-managed sources ────────────────────────────
--
-- free: granted to every User for each free Application.
-- subscription: granted by a Standard subscription, personal or org-wide.
-- Both are written only by the platform, and an admin may disable them.

alter table entitlements add column subscription_id text references subscriptions(id);

alter table entitlements drop constraint entitlements_source_check;
alter table entitlements add constraint entitlements_source_check
  check (source in ('purchase','trial','admin_grant','org_seat','free','subscription'));

alter table entitlements drop constraint entitlements_check1;
alter table entitlements add constraint entitlements_org_source_check
  check (organization_id is null or source in ('org_seat','subscription'));
alter table entitlements add constraint entitlements_org_seat_check
  check (source <> 'org_seat' or organization_id is not null);
alter table entitlements add constraint entitlements_free_check
  check (source <> 'free' or user_id is not null);
alter table entitlements add constraint entitlements_subscription_check
  check ((source = 'subscription') = (subscription_id is not null));

-- A holder has at most one live developer-granted row and, separately,
-- at most one live platform-managed row per Application, so an admin grant
-- and a subscription grant can coexist without either replacing the other.
drop index entitlements_user_app_key;
drop index entitlements_org_app_key;
create unique index entitlements_user_app_key on entitlements (user_id, application_id, test_mode)
  where user_id is not null and status in ('active','disabled') and source not in ('free','subscription');
create unique index entitlements_org_app_key on entitlements (organization_id, application_id, test_mode)
  where organization_id is not null and status in ('active','disabled') and source not in ('free','subscription');
create unique index entitlements_platform_user_key on entitlements (user_id, application_id, test_mode, source)
  where user_id is not null and status in ('active','disabled') and source in ('free','subscription');
create unique index entitlements_platform_org_key on entitlements (organization_id, application_id, test_mode, source)
  where organization_id is not null and status in ('active','disabled') and source in ('free','subscription');
create index entitlements_subscription on entitlements (subscription_id) where subscription_id is not null;

-- ── Audit ─────────────────────────────────────────────────────────────

alter table audit_events drop constraint audit_events_action_check;
alter table audit_events add constraint audit_events_action_check check (action in (
    'user.created','user.updated','user.email_changed','user.suspended','user.reactivated',
    'user.deleted','user.erasure_requested','user.erasure_cancelled','user.erased',
    'user.password_changed','user.password_reset','user.mfa_enabled','user.mfa_disabled','user.mfa_reset',
    'entitlement.granted','entitlement.disabled','entitlement.revoked','entitlement.expired',
    'entitlement.updated','entitlement.member_scope_changed','entitlement.deleted',
    'role.created','role.updated','role.deleted','role.assigned','role.removed',
    'profile.updated','settings.updated',
    'application.created','application.updated','application.review_status_changed',
    'organization.created','organization.updated','organization.member_invited','organization.member_added',
    'organization.member_removed','organization.member_role_changed',
    'subscription.created','subscription.status_changed','subscription.members_changed',
    'tenant.tier_change_requested','tenant.tier_change_request_updated','tenant.tier_changed',
    'api_key.created','api_key.updated','api_key.rotated','api_key.revoked','api_key.restrict_destructive_disabled',
    'webhook.created','webhook.updated','webhook.deleted','webhook.secret_rotated')) not valid;

-- ── Stripe events now point at a subscription, not a Tenant ───────────

alter table stripe_events drop column tenant_id;
alter table stripe_events add column subscription_id text references subscriptions(id);

-- ── Retire Tenant billing ─────────────────────────────────────────────
--
-- Dropping plan also drops the two checks that referenced it
-- (plan = 'enterprise' or tier = 'shared'; plan <> 'starter' or owner_type = 'user').

alter table tenants drop column plan;
alter table tenants drop column stripe_customer_id;
alter table tenants drop column stripe_subscription_id;
alter table tenants drop column subscription_status;
alter table tenants drop column current_period_start;
alter table tenants drop column current_period_end;
alter table tenants drop column cancel_at_period_end;
alter table tenants drop column restricted;
alter table tenants drop column seat_count_synced;
alter table tenants drop column seat_count_synced_at;
