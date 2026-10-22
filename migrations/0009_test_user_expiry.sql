-- 0009_test_user_expiry: test-mode Users are time-limited
-- (Auth → Test-mode Users → Lifetime).
--
-- expires_at is set on every test-mode User and null on every live one.
-- Tokens stop working at expires_at (checked per request); the 5-minute
-- entitlement-sweep then revokes sessions and devices, fires the events,
-- and sets expired_processed_at so it does that exactly once.

-- Numbered after 0008_scim_tokens, which reached production first. The
-- audit action list is the full set, including actions from migrations
-- that may land after this one (self-registered apps, audit exports), so
-- merge order can't drop any.

-- users has forced Row-Level Security; see both modes for the backfill.
select set_config('app.current_test_mode', 'any', true);

alter table users add column expires_at timestamptz, add column expired_processed_at timestamptz;

-- Existing test Users expire when the 30-day purge would have removed
-- them anyway, so nothing changes for them.
update users set expires_at = created_at + interval '30 days' where test_mode;

alter table users
  add constraint users_expires_at_test_mode check ((expires_at is not null) = test_mode),
  add constraint users_expires_at_max check (expires_at <= created_at + interval '30 days');

create index users_test_expiry on users (expires_at) where test_mode and expired_processed_at is null;

alter table devices drop constraint devices_inactive_reason_check;
alter table devices add constraint devices_inactive_reason_check check (inactive_reason in
  ('unregistered','session_revoked','user_suspended','user_deleted','test_user_expired','stale','limit','credential_removed'));

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
