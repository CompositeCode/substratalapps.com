#!/usr/bin/env bash
# Registers app_hub, the Substratal Hub's own Application (Hub spec
# CLIENT-ARCHITECTURE §2): one Application for the web, macOS, Windows,
# Linux, iOS and Android apps. Run it once, as the protected superadmin;
# running it again brings an existing app_hub back to these settings.
#
#   scripts/register-hub.sh              # production
#   SUBSTRATAL_API=http://localhost:8080 scripts/register-hub.sh
#
# It asks for the superadmin's password and current two-factor code, so
# nothing secret lands in shell history. With TOKEN set it skips sign-in.
# It also creates the Web Push (VAPID) credential and prints the public key
# the web Hub needs. APNs (iOS, macOS) and FCM (Android) credentials are
# added later, once their keys exist.
set -euo pipefail

API="${SUBSTRATAL_API:-https://api.substratalapps.com}"
EMAIL="${SUPERADMIN_EMAIL:-adronhall@proton.me}"
DASHBOARD="${DASHBOARD_URL:-https://dashboard.substratalapps.com}"
VERSION="Substratal-Version: 1"
need() { command -v "$1" >/dev/null || { echo "register-hub: $1 is required" >&2; exit 1; }; }
need curl
need jq

post() { # path json [token]
  curl -sS -X POST "$API$1" -H "$VERSION" -H "Content-Type: application/json" \
    ${3:+-H "Authorization: Bearer $3"} -d "$2" -w '\n%{http_code}'
}
call() { # method path [json]
  local args=(-sS -X "$1" "$API$2" -H "$VERSION" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -w '\n%{http_code}')
  [[ -n "${3:-}" ]] && args+=(-d "$3")
  [[ "$1" == POST ]] && args+=(-H "Idempotency-Key: register-hub-$(uuidgen 2>/dev/null || date +%s%N)")
  local out; out=$(curl "${args[@]}")
  STATUS=${out##*$'\n'}; BODY=${out%$'\n'*}
}
fail() { echo "register-hub: $*" >&2; exit 1; }
step() { printf '\n== %s\n' "$*"; }

signed_in_here=false
if [[ -z "${TOKEN:-}" ]]; then
  step "Sign in as $EMAIL"
  printf 'Password: '; IFS= read -rs PW; echo
  out=$(post /auth/login "$(jq -n --arg e "$EMAIL" --arg p "$PW" '{email:$e,password:$p}')"); unset PW
  STATUS=${out##*$'\n'}; BODY=${out%$'\n'*}
  [[ "$STATUS" == 200 ]] || fail "login: HTTP $STATUS: $BODY"
  if [[ $(jq -r '.mfa_required // false' <<<"$BODY") == true ]]; then
    MFA=$(jq -r .mfa_token <<<"$BODY")
    printf 'Two-factor code: '; IFS= read -r CODE
    out=$(post /auth/mfa/verify "$(jq -n --arg t "$MFA" --arg c "$CODE" '{mfa_token:$t,code:$c}')")
    STATUS=${out##*$'\n'}; BODY=${out%$'\n'*}
    [[ "$STATUS" == 200 ]] || fail "two-factor: HTTP $STATUS: $BODY"
  fi
  TOKEN=$(jq -r .access_token <<<"$BODY")
  signed_in_here=true
fi
call GET /users/me
[[ "$STATUS" == 200 ]] || fail "GET /users/me: HTTP $STATUS: $BODY"
ME=$(jq -r .id <<<"$BODY")
echo "signed in as $ME"

# Theme ids match the Hub spec's shared/themes/themes.json.
APP=$(jq -n --arg owner "$ME" --arg dash "$DASHBOARD" '{
  name: "Substratal Hub",
  description: "Your Substratal apps, account, inbox and administration, on the web, macOS, Windows, Linux, iOS and Android.",
  launch_url: $dash,
  support_url: "https://compositecode.github.io/substratalapps-applications-specs/",
  email_from_name: "Substratal",
  visibility: "internal",
  access_tier: "free",
  receives_platform_notifications: true,
  review_status: "approved",
  owner_user_id: $owner,
  settings_schema: {
    type: "object",
    properties: {
      light_theme: { type: "string", enum: ["teal-ledger", "indigo-root", "moss-paper", "coral-clay", "spore-pink"], default: "moss-paper" },
      dark_theme:  { type: "string", enum: ["night-soil", "deep-teal", "graphite-spore", "forest-floor", "ember-clay"], default: "forest-floor" }
    }
  }
}')

step "Register app_hub"
call POST /applications "$(jq '. + {slug: "hub"}' <<<"$APP")"
if [[ "$STATUS" == 201 ]]; then
  echo "created $(jq -r .id <<<"$BODY")"
elif [[ "$STATUS" == 409 && $(jq -r .error.code <<<"$BODY") == slug_taken ]]; then
  call PATCH /applications/app_hub "$APP"
  [[ "$STATUS" == 200 ]] || fail "update app_hub: HTTP $STATUS: $BODY"
  echo "app_hub already existed; updated it to these settings"
else
  fail "register app_hub: HTTP $STATUS: $BODY"
fi
jq '{id, slug, name, visibility, access_tier, review_status, receives_platform_notifications, launch_url}' <<<"$BODY"

step "Web Push (VAPID) credential"
call GET /applications/app_hub/push-credentials/web_push
if [[ "$STATUS" == 404 ]]; then
  call POST /applications/app_hub/push-credentials '{"provider":"web_push","subject":"mailto:security@substratalapps.com"}'
  [[ "$STATUS" == 201 ]] || fail "create web_push credential: HTTP $STATUS: $BODY"
  echo "created"
elif [[ "$STATUS" != 200 ]]; then
  fail "read web_push credential: HTTP $STATUS: $BODY"
else
  echo "already set up"
fi
VAPID=$(jq -r .vapid_public_key <<<"$BODY")
echo "vapid_public_key: $VAPID"

step "Check: you have access to app_hub"
call GET "/users/$ME/apps/app_hub/effective-permissions"
[[ "$STATUS" == 200 ]] || fail "effective-permissions: HTTP $STATUS: $BODY"
jq '{allowed, entitlement_status}' <<<"$BODY"

if $signed_in_here; then
  curl -sS -o /dev/null -X POST "$API/auth/logout" -H "$VERSION" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{}'
fi

cat <<EOF

Done. app_hub is registered.
- Give the web Hub this VAPID public key (applicationServerKey for PushManager.subscribe()):
  $VAPID
- Next, when the keys exist: APNs (iOS + macOS, topic com.substratalapps.hub) and FCM (Android),
  with POST /applications/app_hub/push-credentials.
EOF
