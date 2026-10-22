-- 0008_scim_tokens: SCIM provisioning (API Reference → SCIM).
--
-- One SCIM bearer token per Organization, for its identity provider. Only
-- the SHA-256 of the secret is stored, like API Keys.
--
-- Numbered after 0005_test_user_expiry, 0006_self_registered_apps, and
-- 0007_audit_exports, which ship first; the action list below keeps their
-- user.expired and audit.exported.

create table scim_tokens (
  id text primary key,
  organization_id text not null unique references organizations(id),
  secret_hash text not null unique,
  created_by text not null references users(id),
  test_mode boolean not null default false,
  created_at timestamptz not null default now(),
  last_used_at timestamptz
);

-- No mode policy, like api_keys: a token is resolved by its hash before
-- the mode is known, and its test_mode is the mode its calls run in.

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
