#!/usr/bin/env bash

set -euo pipefail

namespace="toolhive-demo"
username="${1:-support-bot@example.com}"
password="${DEMO_PASSWORD:-password}"
client_secret="${DEX_CLIENT_SECRET:-toolhive-demo-secret}"
token_url="http://127.0.0.1:5556/dex/token"
discovery_url="http://127.0.0.1:5556/dex/.well-known/openid-configuration"
tmp_dir="$(mktemp -d)"
port_forward_pid=""

cleanup() {
  if [[ -n "${port_forward_pid}" ]]; then
    kill "${port_forward_pid}" 2>/dev/null || true
    wait "${port_forward_pid}" 2>/dev/null || true
  fi
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

if ! curl --silent --fail --connect-timeout 2 --max-time 5 \
  "${discovery_url}" >/dev/null 2>&1; then
  kubectl -n "${namespace}" port-forward service/dex 5556:5556 \
    >"${tmp_dir}/port-forward.log" 2>&1 &
  port_forward_pid=$!

  for _ in {1..30}; do
    curl --silent --fail --connect-timeout 2 --max-time 2 \
      "${discovery_url}" >/dev/null 2>&1 && break
    if ! kill -0 "${port_forward_pid}" 2>/dev/null; then
      cat "${tmp_dir}/port-forward.log" >&2
      exit 1
    fi
    sleep 0.5
  done
  if ! curl --silent --fail --connect-timeout 2 --max-time 5 \
    "${discovery_url}" >/dev/null 2>&1; then
    cat "${tmp_dir}/port-forward.log" >&2
    echo "Dex did not become reachable" >&2
    exit 1
  fi
fi

response="$(curl --silent --show-error --fail-with-body \
  --connect-timeout 2 --max-time 10 \
  --user "toolhive-demo:${client_secret}" \
  --data-urlencode grant_type=password \
  --data-urlencode "username=${username}" \
  --data-urlencode "password=${password}" \
  --data-urlencode "scope=openid profile email groups" \
  "${token_url}")"

jq --exit-status --raw-output '.access_token' <<<"${response}"
