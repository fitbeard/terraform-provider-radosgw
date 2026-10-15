#!/usr/bin/env bash
# =============================================================================
# jwt-token.sh — mint a JWT from the local OIDC provider (scripts/compose.yml)
# =============================================================================
# Drives the OIDC Authorization Code + PKCE flow against the dev OIDC provider
# and prints the resulting **id_token** (a JWT) to stdout. The token is intended
# for RGW STS AssumeRoleWithWebIdentity testing.
#
# Usage:
#   ./scripts/jwt-token.sh              # prints the id_token (JWT) to stdout
#   DEBUG=1 ./scripts/jwt-token.sh      # also dumps the token response + claims to stderr
#   TOKEN=$(./scripts/jwt-token.sh)     # capture for use in a test
#
# Overridable via environment (defaults match scripts/compose.yml):
#   OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URI,
#   OIDC_USERNAME, OIDC_PASSWORD
# =============================================================================
set -euo pipefail

ISS="${OIDC_ISSUER:-http://localhost:9080}"
CID="${OIDC_CLIENT_ID:-test-client}"
SEC="${OIDC_CLIENT_SECRET:-test-secret}"
RU="${OIDC_REDIRECT_URI:-http://localhost:3000/callback}"
USERNAME="${OIDC_USERNAME:-radosgw@terraform}"
PASSWORD="${OIDC_PASSWORD:-test-password}"

for bin in curl jq openssl; do
  command -v "$bin" >/dev/null 2>&1 || { echo "jwt-token.sh: missing required tool: $bin" >&2; exit 1; }
done

JAR=$(mktemp)
TOK=$(mktemp)
trap 'rm -f "$JAR" "$TOK"' EXIT

# PKCE verifier + S256 challenge.
VERIFIER=$(openssl rand -hex 32)
CHALLENGE=$(printf %s "$VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')

# follow prints the Location header of a response, carrying the session cookie jar.
# A connection failure yields an empty string (not a set -e abort) so the emptiness
# checks below can report a clear, actionable error.
follow() { curl -s -o /dev/null -b "$JAR" -c "$JAR" -w '%{redirect_url}' "$@" || true; }

# 1. /auth -> login interaction
LOGIN=$(follow -G "$ISS/auth" \
  --data-urlencode response_type=code --data-urlencode client_id="$CID" \
  --data-urlencode redirect_uri="$RU" --data-urlencode "scope=openid profile email" \
  --data-urlencode state=s1 --data-urlencode nonce=n1 \
  --data-urlencode code_challenge="$CHALLENGE" --data-urlencode code_challenge_method=S256)
[ -n "$LOGIN" ] || { echo "jwt-token.sh: /auth did not return a login interaction (is the OIDC provider up at $ISS?)" >&2; exit 1; }

# 2. login -> resume -> consent interaction
RESUME=$(follow -d email="$USERNAME" -d password="$PASSWORD" "$LOGIN/login")
CONSENT=$(follow "$RESUME")

# 3. approve consent -> resume -> redirect carrying ?code=
RESUME=$(follow -d '' "$CONSENT/confirm")
CB=$(follow "$RESUME")
CODE=$(sed -n 's/.*[?&]code=\([^&]*\).*/\1/p' <<<"$CB")
[ -n "$CODE" ] || { echo "jwt-token.sh: no authorization code returned (callback: ${CB:-<empty>})" >&2; exit 1; }

# 4. exchange the code for tokens
curl -s -u "$CID:$SEC" -d grant_type=authorization_code -d code="$CODE" \
  -d redirect_uri="$RU" -d code_verifier="$VERIFIER" "$ISS/token" > "$TOK" || true

ID_TOKEN=$(jq -r '.id_token // empty' "$TOK")
[ -n "$ID_TOKEN" ] || { echo "jwt-token.sh: token endpoint returned no id_token: $(cat "$TOK")" >&2; exit 1; }

if [ "${DEBUG:-}" = "1" ]; then
  echo "=== token response ===" >&2
  jq . "$TOK" >&2
  echo "=== id_token claims ===" >&2
  jq -R 'split(".")[1] | gsub("-";"+") | gsub("_";"/") | @base64d | fromjson' <<<"$ID_TOKEN" >&2
fi

printf '%s\n' "$ID_TOKEN"
