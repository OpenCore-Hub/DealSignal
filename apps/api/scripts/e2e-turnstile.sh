# Cloudflare Turnstile helpers for API e2e scripts.
# Empty site key → omit token (local / MSW). Dummy 1x000000 keys → dummy token.

e2e_turnstile_field() {
  local base="${1:?base url required}"
  local key=""
  key=$(curl -fsS "$base/api/auth/captcha" 2>/dev/null | jq -r '.turnstile_site_key // empty' || true)
  if [[ -z "$key" || "$key" == "null" ]]; then
    return 0
  fi
  local token="${E2E_TURNSTILE_TOKEN:-}"
  if [[ "$key" == 1x000000* ]]; then
    token="${token:-XXXX.DUMMY.TOKEN.XXXX}"
  fi
  if [[ -z "$token" ]]; then
    echo "Turnstile is enabled with a real site key. Set E2E_TURNSTILE_TOKEN or use Cloudflare dummy keys for API e2e." >&2
    return 1
  fi
  printf ',"turnstile_token":"%s"' "$token"
}
