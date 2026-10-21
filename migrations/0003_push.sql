-- 0003_push: push notifications and the notification inbox (API Reference →
-- Push Notifications; Database Schema → push_credentials, devices,
-- notifications, notification_deliveries). Expand-only.

-- ── Push credentials: Application configuration, shared by both modes ──

create table push_credentials (
  id text primary key,
  application_id text not null references applications(id),
  tenant_id text not null references tenants(id),
  provider text not null check (provider in ('apns','fcm','web_push')),
  config jsonb not null,
  secret_ciphertext text not null,
  status text not null default 'active' check (status in ('active','failed')),
  failure_reason text,
  failed_at timestamptz,
  last_used_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  version integer not null default 1,
  unique (application_id, provider),
  check ((status = 'failed') = (failed_at is not null))
);

-- ── Devices ───────────────────────────────────────────────────────────

create table devices (
  id text primary key,
  user_id text not null references users(id),
  application_id text not null references applications(id),
  tenant_id text not null references tenants(id),
  session_id text references sessions(id) on delete set null,
  provider text not null check (provider in ('apns','fcm','web_push')),
  environment text check (environment in ('production','sandbox')),
  token_hash text not null,
  token_ciphertext text not null,
  token_hint text not null,
  name text check (char_length(name) <= 100),
  app_version text check (char_length(app_version) <= 50),
  status text not null default 'active' check (status in ('active','inactive')),
  inactive_reason text check (inactive_reason in
    ('unregistered','session_revoked','user_suspended','user_deleted','stale','limit','credential_removed')),
  deactivated_at timestamptz,
  last_registered_at timestamptz not null default now(),
  last_delivery_at timestamptz,
  test_mode boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  version integer not null default 1,
  check ((provider = 'apns') = (environment is not null)),
  check ((status = 'inactive') = (inactive_reason is not null)),
  check ((status = 'inactive') = (deactivated_at is not null))
);
create unique index devices_token_key on devices (application_id, provider, token_hash, test_mode);
create index devices_user_app_active on devices (user_id, application_id) where status = 'active';
create index devices_session on devices (session_id) where session_id is not null;
create index devices_stale on devices (last_registered_at) where status = 'active';
create index devices_inactive on devices (deactivated_at) where status = 'inactive';

-- ── Notifications: the unit of sending, and the inbox item ────────────

create table notifications (
  id text primary key,
  user_id text not null references users(id),
  application_id text references applications(id),
  tenant_id text references tenants(id),
  source text not null check (source in ('application','platform')),
  category text not null check (category ~ '^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$' and char_length(category) <= 64),
  priority text not null default 'normal' check (priority in ('normal','high')),
  title text not null check (char_length(title) between 1 and 120),
  body text check (char_length(body) <= 1000),
  data jsonb not null default '{}',
  action_url text check (char_length(action_url) <= 2048),
  badge integer check (badge between 0 and 99999),
  sound text check (sound = 'default'),
  collapse_key text check (char_length(collapse_key) <= 64),
  ttl_seconds integer not null default 86400 check (ttl_seconds between 0 and 2419200),
  push_requested boolean not null default true,
  push_status text not null check (push_status in ('queued','sent','failed','skipped')),
  skip_reason text check (skip_reason in
    ('push_not_requested','push_disabled','no_devices','no_credentials','credential_failed','expired')),
  push_devices_targeted integer not null default 0,
  push_devices_sent integer not null default 0,
  push_devices_failed integer not null default 0,
  sender_type text not null check (sender_type in ('api_key','user','system')),
  sender_id text not null,
  read_at timestamptz,
  archived_at timestamptz,
  test_mode boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  version integer not null default 1,
  check ((source = 'platform') = (application_id is null)),
  check ((application_id is null) = (tenant_id is null)),
  check ((push_status = 'skipped') = (skip_reason is not null)),
  check (source = 'platform' or category !~ '^(security|account|billing)\.')
);
create index notifications_inbox on notifications (user_id, created_at desc, id) where archived_at is null;
create index notifications_user_app on notifications (user_id, application_id, created_at desc);
create index notifications_unread on notifications (user_id) where read_at is null and archived_at is null;
create index notifications_created on notifications (created_at);

-- ── Deliveries: the push outbox, one per (notification, device) ───────

create table notification_deliveries (
  id text primary key,
  notification_id text not null references notifications(id) on delete cascade,
  device_id text references devices(id) on delete set null,
  provider text not null check (provider in ('apns','fcm','web_push')),
  status text not null default 'pending' check (status in ('pending','sent','failed','skipped')),
  attempts int not null default 0 check (attempts between 0 and 4),
  provider_status int,
  error text,
  next_attempt_at timestamptz,
  sent_at timestamptz,
  test_mode boolean not null default false,
  created_at timestamptz not null default now()
);
create index notification_deliveries_due on notification_deliveries (next_attempt_at) where status = 'pending';
create index notification_deliveries_notification on notification_deliveries (notification_id);
create index notification_deliveries_device on notification_deliveries (device_id) where device_id is not null;

-- ── Triggers and test/live isolation ──────────────────────────────────

do $$
declare t text;
begin
  foreach t in array array['push_credentials','devices','notifications'] loop
    execute format('create trigger %I before update on %I for each row execute function substratal_util.touch_updated_at()',
                   t || '_touch_updated_at', t);
    execute format('create trigger %I before update on %I for each row execute function substratal_util.bump_version()',
                   t || '_bump_version', t);
  end loop;
  -- push_credentials is Application configuration, like applications:
  -- one credential serves both modes, so it has no mode policy.
  foreach t in array array['devices','notifications','notification_deliveries'] loop
    execute format('alter table %I enable row level security', t);
    execute format('alter table %I force row level security', t);
    execute format('create policy %I on %I using (substratal_util.mode_visible(test_mode)) with check (substratal_util.mode_visible(test_mode))',
                   t || '_mode', t);
  end loop;
end $$;

-- ── Applications opt in to platform alert pushes ──────────────────────

alter table applications add column receives_platform_notifications boolean not null default false;

-- ── notifications.send: superadmin holds every platform permission ────

update roles set permissions = array_append(permissions, 'notifications.send')
  where id = 'role_platform_superadmin' and not ('notifications.send' = any(permissions));

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
    'webhook.created','webhook.updated','webhook.deleted','webhook.secret_rotated',
    'push_credential.created','push_credential.updated','push_credential.deleted','device.deleted')) not valid;
