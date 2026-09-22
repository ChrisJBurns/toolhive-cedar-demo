#!/usr/bin/env bash

set -euo pipefail

namespace="toolhive-demo"
dex_pid=""
vmcp_pid=""
tmp_dir="$(mktemp -d)"

cleanup() {
  if [[ -n "${dex_pid}" ]]; then
    kill "${dex_pid}" 2>/dev/null || true
    wait "${dex_pid}" 2>/dev/null || true
  fi
  if [[ -n "${vmcp_pid}" ]]; then
    kill "${vmcp_pid}" 2>/dev/null || true
    wait "${vmcp_pid}" 2>/dev/null || true
  fi
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT INT TERM

kubectl -n "${namespace}" port-forward service/dex 5556:5556 \
  >"${tmp_dir}/dex.log" 2>&1 &
dex_pid=$!

kubectl -n "${namespace}" port-forward service/vmcp-cedar-demo 4483:4483 \
  >"${tmp_dir}/vmcp.log" 2>&1 &
vmcp_pid=$!

sleep 1
if ! kill -0 "${dex_pid}" 2>/dev/null || ! kill -0 "${vmcp_pid}" 2>/dev/null; then
  cat "${tmp_dir}/dex.log" >&2
  cat "${tmp_dir}/vmcp.log" >&2
  echo "One or more port-forwards failed to start" >&2
  exit 1
fi

echo "Dex available at http://127.0.0.1:5556/dex"
echo "vMCP available at http://127.0.0.1:4483/mcp"
echo "Press Ctrl-C to stop both port-forwards"

wait
