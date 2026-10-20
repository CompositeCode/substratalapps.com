-- 0004_protected_superadmin: a superadmin that can never be removed.
--
-- A protected User is the platform's owner account. Every deploy re-asserts
-- it (cmd/migrate, SUPERADMIN_EMAIL), and the database refuses anything
-- that would lock it out, for every caller, including a direct SQL session:
-- it stays active and undeleted, keeps its email and live mode, keeps the
-- superadmin Role, and can't be erased.
--
-- The guard can be lifted for one transaction with
-- SET LOCAL app.protected_user_maintenance = 'on' (ops only, never the API),
-- for example to hand ownership to a different account.
--
-- Numbered 0004 so it can ship ahead of 0003_push; the two are independent.

alter table users add column protected boolean not null default false;

create function substratal_util.protected_user_maintenance() returns boolean
language sql stable as $$
  select coalesce(current_setting('app.protected_user_maintenance', true), '') = 'on'
$$;

create function substratal_util.protect_users() returns trigger language plpgsql as $$
begin
  if substratal_util.protected_user_maintenance() then
    if tg_op = 'DELETE' then return old; end if;
    return new;
  end if;
  if tg_op = 'DELETE' then
    if old.protected then
      raise exception 'user % is protected and can''t be deleted', old.id using errcode = '42501';
    end if;
    return old;
  end if;
  if old.protected and (not new.protected or new.status <> 'active' or new.deleted_at is not null
      or new.email is distinct from old.email or new.test_mode <> old.test_mode) then
    raise exception 'user % is protected: it must stay an active, undeleted superadmin with the same email', old.id
      using errcode = '42501';
  end if;
  return new;
end $$;

create trigger users_protected before update or delete on users
  for each row execute function substratal_util.protect_users();

create function substratal_util.protect_superadmin_role() returns trigger language plpgsql as $$
begin
  if not substratal_util.protected_user_maintenance()
     and old.role_id = 'role_platform_superadmin'
     and (tg_op = 'DELETE' or new.role_id <> old.role_id or new.user_id <> old.user_id)
     and exists (select 1 from users u where u.id = old.user_id and u.protected) then
    raise exception 'user % is protected and keeps the superadmin role', old.user_id using errcode = '42501';
  end if;
  if tg_op = 'DELETE' then return old; end if;
  return new;
end $$;

create trigger user_role_assignments_protected before update or delete on user_role_assignments
  for each row execute function substratal_util.protect_superadmin_role();

create function substratal_util.protect_erasure() returns trigger language plpgsql as $$
begin
  if not substratal_util.protected_user_maintenance()
     and new.status = 'scheduled'
     and exists (select 1 from users u where u.id = new.user_id and u.protected) then
    raise exception 'user % is protected and can''t be erased', new.user_id using errcode = '42501';
  end if;
  return new;
end $$;

create trigger erasure_requests_protected before insert or update on erasure_requests
  for each row execute function substratal_util.protect_erasure();
