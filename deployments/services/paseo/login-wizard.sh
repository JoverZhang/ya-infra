#!/usr/bin/env bash
# Guides device login and checks authentication for one Paseo instance.
set -euo pipefail
INSTANCE="${1:?Usage: login-wizard.sh INSTANCE}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../../lib/wizard.sh"
TOTAL_STAGES=2

banner "Paseo ${INSTANCE} — Codex account login"

stage "Create an independent login session for this instance"
say "Sign in with the same ChatGPT account separately for each instance."
say "Codex saves authentication to /home/paseo/.codex/auth.json inside this instance."
step "Open the URL shown by Codex and enter the one-time device code to complete login."
step "You do not need to copy a token or change the host's existing Codex login."
open_url "https://auth.openai.com/codex/device"
python3 "$SCRIPT_DIR/manage" device-login "$INSTANCE"

stage "Check this instance's authentication status"
python3 "$SCRIPT_DIR/manage" auth-status "$INSTANCE"
note "This instance's login cache persists across redeployments."

finish
