-- 0011_audit_exports: asynchronous Audit Event exports
-- (API Reference → Audit → Exporting).
--
-- A request stores the caller's forced scope and filters; the jobs worker
-- writes the file to private storage and marks it ready. Rows and files
-- are deleted 7 days after the request.
--
-- Numbered after 0008_scim_tokens (in production), 0009_test_user_expiry,
-- and 0010_self_registered_apps. The audit action list is the full set,
-- so whichever order those land in, no action is dropped.

create table audit_exports (
  id text primary key,
  requested_by_type text not null check (requested_by_type in ('user','api_key')),
  requested_by_id text not null,
  scope jsonb not null,          -- the forced scope: {application_id | target_user_id | tenant_id}, or {}
  filters jsonb not null,
  format text not null check (format in ('ndjson','csv')),
  status text not null default 'pending' check (status in ('pending','running','ready','failed')),
  row_count integer,
  object_key text,
  error text,
  test_mode boolean not null default false,
  created_at timestamptz not null default now(),
  started_at timestamptz,
  completed_at timestamptz,
  expires_at timestamptz not null,
  check ((status = 'ready') = (object_key is not null and row_count is not null)),
  check ((status = 'failed') = (error is not null))
);
create index audit_exports_due on audit_exports (created_at) where status in ('pending','running');

alter table audit_exports enable row level security;
alter table audit_exports force row level security;
create policy audit_exports_mode on audit_exports
  using (substratal_util.mode_visible(test_mode)) with check (substratal_util.mode_visible(test_mode));

alter table audit_events drop constraint audit_events_action_check;
alter table audit_events add constraint audit_events_action_check check (action in (
    'user.created','user.updated','user.email_changed','user.suspended','user.reactivated',
    'user.deleted','user.expired','user.erasure_requested','user.erasure_cancelled','user.erased',
    'user.password_changed','user.password_reset','user.mfa_enabled','user.mfa_disabled','user.mfa_reset',
    'entitlement.granted','entitlement.disabled','entitlement.revoked','entitlement.expired',
    'entitlement.updated','entitlement.member_scope_changed','entitlement.deleted',
    'role.created','role.updated','role.deleted','role.assigned','role.removed',
    'profile.updated','settings.updated',
    'application.created','application.updated','application.review_status_changed',
    'organization.created','organization.updated','organization.member_invited','organization.member_added',
    'organization.member_removed','organization.member_role_changed',
    'organization.scim_token_created','organization.scim_token_revoked',
    'subscription.created','subscription.status_changed','subscription.members_changed',
    'tenant.tier_change_requested','tenant.tier_change_request_updated','tenant.tier_changed',
    'api_key.created','api_key.updated','api_key.rotated','api_key.revoked','api_key.restrict_destructive_disabled',
    'webhook.created','webhook.updated','webhook.deleted','webhook.secret_rotated',
    'push_credential.created','push_credential.updated','push_credential.deleted','device.deleted',
    'audit.exported')) not valid;
