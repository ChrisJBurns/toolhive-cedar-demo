#!/usr/bin/env bash

set -euo pipefail

token_file="${1:?Usage: create-github-token.sh TOKEN_FILE}"

printf 'GitHub token: '
IFS= read -r -s github_token
printf '\n'

github_token="$(printf '%s' "$github_token" | tr -d '\r\n')"
if [[ -z "$github_token" ]]; then
  echo "GitHub token must not be empty" >&2
  exit 1
fi

mkdir -p "$(dirname "$token_file")"
umask 077
printf '%s' "$github_token" > "$token_file"
chmod 600 "$token_file"
unset github_token

echo "Saved GitHub token to $token_file"
