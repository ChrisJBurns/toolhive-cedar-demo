#!/usr/bin/env bash

set -euo pipefail

token_file="${1:?Usage: create-github-token.sh TOKEN_FILE}"

if [[ ! -s "$token_file" ]]; then
  echo "Create $token_file containing a GitHub token" >&2
  exit 1
fi

github_token="$(tr -d '\r\n' < "$token_file")"
if [[ -z "$github_token" ]]; then
  echo "GitHub token must not be empty" >&2
  exit 1
fi

chmod 600 "$token_file"
printf '%s' "$github_token" > "$token_file"
unset github_token

echo "Cleaned GitHub token file $token_file"
