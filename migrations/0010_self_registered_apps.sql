-- 0010_self_registered_apps: self-service Application registration
-- (Applications → The review lifecycle).
--
-- self_registered marks an Application a developer submitted themselves,
-- rather than one staff created (concierge onboarding). Such an
-- Application must use the hosted sign-in flow: embedded app-token
-- issuance would hand a third party the User's Substratal password
-- (Auth → Getting an app token).
--
-- Numbered after 0008_scim_tokens (in production) and 0009_test_user_expiry.

alter table applications add column self_registered boolean not null default false;
