-- 0012_sessions_revoked: the user.sessions_revoked audit action, written
-- by POST /users/{id}/sessions/revoke-all (Auth → Sessions).
--
-- The audit action list is the full set, as in 0011, so no action is
-- dropped. sessions.revoked_reason is free text, so revoked_all needs no
-- change there.

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
    'audit.exported','user.sessions_revoked')) not valid;
