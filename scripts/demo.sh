#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
namespace="toolhive-demo"
username="${1:-alice@example.com}"
endpoint="http://127.0.0.1:4483/mcp"
tmp_dir="$(mktemp -d)"
port_forward_pid=""
session_id=""

cleanup() {
  if [[ -n "${port_forward_pid}" ]]; then
    kill "${port_forward_pid}" 2>/dev/null || true
    wait "${port_forward_pid}" 2>/dev/null || true
  fi
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

pretty_print() {
  local file="$1"
  if jq empty "${file}" >/dev/null 2>&1; then
    jq . "${file}"
  else
    sed -n 's/^data: //p' "${file}" | jq . 2>/dev/null || cat "${file}"
  fi
}

echo "Getting a Dex token for ${username}..."
token="$("${script_dir}/get-token.sh" "${username}")"

if ! curl --silent --output /dev/null --connect-timeout 1 --max-time 2 "${endpoint}"; then
  kubectl -n "${namespace}" port-forward service/vmcp-cedar-demo 4483:4483 \
    >"${tmp_dir}/port-forward.log" 2>&1 &
  port_forward_pid=$!
  for _ in {1..30}; do
    curl --silent --output /dev/null --connect-timeout 1 --max-time 2 "${endpoint}" && break
    if ! kill -0 "${port_forward_pid}" 2>/dev/null; then
      cat "${tmp_dir}/port-forward.log" >&2
      exit 1
    fi
    sleep 0.5
  done
  if ! curl --silent --output /dev/null --connect-timeout 1 --max-time 2 "${endpoint}"; then
    cat "${tmp_dir}/port-forward.log" >&2
    echo "vMCP did not become reachable" >&2
    exit 1
  fi
fi

echo "Initializing an authenticated MCP session..."
http_status="$(curl --silent --show-error \
  --connect-timeout 2 --max-time 10 \
  --dump-header "${tmp_dir}/headers" \
  --output "${tmp_dir}/initialize.json" \
  --write-out '%{http_code}' \
  --header "Authorization: Bearer ${token}" \
  --header 'Accept: application/json, text/event-stream' \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"cedar-dex-demo","version":"1.0"}}}' \
  "${endpoint}")"

if [[ "${http_status}" != "200" ]]; then
  echo "Initialize failed with HTTP ${http_status}" >&2
  pretty_print "${tmp_dir}/initialize.json" >&2
  exit 1
fi

session_id="$(awk 'tolower($1) == "mcp-session-id:" {gsub("\r", "", $2); print $2}' "${tmp_dir}/headers" | tail -1)"
if [[ -z "${session_id}" ]]; then
  echo "The server did not return an MCP session ID" >&2
  exit 1
fi

curl --silent --show-error --output /dev/null \
  --connect-timeout 2 --max-time 10 \
  --header "Authorization: Bearer ${token}" \
  --header "Mcp-Session-Id: ${session_id}" \
  --header 'MCP-Protocol-Version: 2025-06-18' \
  --header 'Accept: application/json, text/event-stream' \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  "${endpoint}"

echo
echo "Tools visible through Cedar:"
curl --silent --show-error \
  --connect-timeout 2 --max-time 10 \
  --output "${tmp_dir}/tools.json" \
  --header "Authorization: Bearer ${token}" \
  --header "Mcp-Session-Id: ${session_id}" \
  --header 'MCP-Protocol-Version: 2025-06-18' \
  --header 'Accept: application/json, text/event-stream' \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  "${endpoint}"
pretty_print "${tmp_dir}/tools.json"
