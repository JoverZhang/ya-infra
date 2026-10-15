#!/usr/bin/env bash
# Guides management-token setup while keeping credentials outside Git.
set -euo pipefail
infra_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$infra_root/lib/wizard.sh"
TOTAL_STAGES=1

_existing() {
  [[ -f "$ENV_FILE" ]] || return 1
  local line; line=$(grep -E "^${1}=" "$ENV_FILE" | tail -n1) || return 1
  printf '%s' "${line#*=}"
}

ask_secret() {
  local key="$1" prompt="$2" current input
  current=$(_existing "$key" || true)
  if [[ -n "$current" ]]; then
    printf '  %s%s%s %s[Enter keeps current]%s ' "$BOLD" "$prompt" "$RESET" "$DIM" "$RESET"
  else
    printf '  %s%s%s ' "$BOLD" "$prompt" "$RESET"
  fi
  read -rs input || true
  printf '\n'
  [[ -z "$input" && -n "$current" ]] && input="$current"
  printf -v "$key" '%s' "$input"
}

write_env() {
  local key="$1" value="$2" tmp
  touch "$ENV_FILE"
  tmp=$(mktemp)
  grep -vE "^${key}=" "$ENV_FILE" > "$tmp" || true
  printf '%s=%s\n' "$key" "$value" >> "$tmp"
  mv "$tmp" "$ENV_FILE"
  WRITTEN_ENV+=("$key")
  printf '  %s✓ wrote%s %s → %s\n' "$GREEN" "$RESET" "$key" "$ENV_FILE"
}

umask 077
ENV_FILE="$(python3 - "$infra_root" <<'PY_PATH'
import sys
sys.path.insert(0, sys.argv[1])
from gateway.cloudflare import credentials_file
print(credentials_file())
PY_PATH
)"
credential_directory="$(dirname "$ENV_FILE")"
mkdir -p "$credential_directory"
chmod 700 "$credential_directory"
[[ ! -f "$ENV_FILE" ]] || chmod 600 "$ENV_FILE"

if [[ -f "$ENV_FILE" ]] && python3 "$infra_root/gateway/manage" check-credentials >/dev/null 2>&1; then
  say "Existing management credential can read the configured account, tunnel and DNS."
else
  banner "Cloudflare public gateway — management credential"
  stage "Create or repair a scoped management API token"
  open_url "https://dash.cloudflare.com/profile/api-tokens/"
  step "For an existing infra-public-gateway token, select Edit. Otherwise Create Token → Create Custom Token."
  step "Add Account → Cloudflare One Connector: cloudflared → Edit."
  note "If your dashboard uses the older Cloudflare Tunnel → Edit permission, it is also accepted."
  step "Add a separate permission row: Zone → DNS → Edit."
  step "Add another permission row: Zone → Zone → Read."
  note "Choose Zone itself, not Zone Custom Assets. Zone Read ID: c8fed203ed3043cba015a93ad1616f1f."
  gateway_account="$(python3 - "$infra_root" <<'PY_ACCOUNT'
import sys
sys.path.insert(0, sys.argv[1])
from gateway.model import public
print(public()['account_id'])
PY_ACCOUNT
)"
  gateway_zone="$(python3 - "$infra_root" <<'PY_ZONE'
import sys
sys.path.insert(0, sys.argv[1])
from gateway.model import public
print(public()['zone'])
PY_ZONE
)"
  step "Restrict Account Resources to the account with ID $gateway_account."
  step "Restrict Zone Resources to the specific zone $gateway_zone."
  step "Remove the entire optional Client IP Address Filtering rule, or allow this server's outgoing IP."
  note "Do not just clear its input field. An empty Include/in rule can still block requests."
  note "If check-credentials reports an IP block, allow the exact IP/CIDR shown in that result and save."
  step "Review and save edits, or Create Token and copy its secret. Name new tokens infra-public-gateway."
  note "Use the management API token, rather than the existing cloudflared connector token."
  ask_secret CLOUDFLARE_API_TOKEN "Paste the API token (hidden); Enter keeps the existing secret after permission edits:"
  if [[ ${#CLOUDFLARE_API_TOKEN} -lt 20 || "$CLOUDFLARE_API_TOKEN" =~ [[:space:]] ]]; then
    warn "The token is empty or malformed. Nothing was saved."
    exit 1
  fi
  write_env CLOUDFLARE_API_TOKEN "$CLOUDFLARE_API_TOKEN"
  chmod 600 "$ENV_FILE"
  unset CLOUDFLARE_API_TOKEN
fi

python3 "$infra_root/gateway/manage" check-credentials
python3 "$infra_root/gateway/manage" plan
finish
say "Credential saved outside Git. Review the plan, then run: just gateway apply <route>"
