#!/usr/bin/env bash

set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
schema="${root_dir}/analysis/toolhive.cedarschema"
policies="${root_dir}/analysis/toolhive.cedar"
environments="${root_dir}/analysis/request-environments.json"
transitions="${root_dir}/.state/exfiltration-transitions.json"
results="${root_dir}/.state/exfiltration-analysis.json"
manifest="${root_dir}/policies/demo/20-combined-access.yaml"
toolhive_manifest="${root_dir}/manifests/20-toolhive.yaml"
dex_manifest="${root_dir}/manifests/10-dex.yaml"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

sed -n "s/^      - '\(.*\)'$/\1/p" "${manifest}" > "${tmp_dir}/manifest.cedar"
sed '/^[[:space:]]*\/\//d; /^[[:space:]]*$/d' "${policies}" > "${tmp_dir}/analysis.cedar"
if ! diff -u "${tmp_dir}/manifest.cedar" "${tmp_dir}/analysis.cedar"; then
  echo "analysis/toolhive.cedar does not match policies/demo/20-combined-access.yaml" >&2
  exit 1
fi

awk '
  /^[[:space:]]+filter:$/ { in_filter = 1; next }
  in_filter && /^[[:space:]]+- / {
    sub(/^[[:space:]]+- /, "")
    print
    next
  }
  in_filter { in_filter = 0 }
' "${toolhive_manifest}" | sort -u > "${tmp_dir}/exposed-tools"

jq -r '[.internal_data_readers[], .public_internet_writers[]] | .[].name' \
  "${environments}" | while IFS= read -r tool; do
    if ! grep -Fx -- "${tool}" "${tmp_dir}/exposed-tools" >/dev/null; then
      echo "classified tool '${tool}' is not exposed by manifests/20-toolhive.yaml" >&2
      exit 1
    fi
  done

awk '
  /- email: alice@example.com$/ { in_alice = 1 }
  in_alice && /^[[:space:]]+groups:$/ { in_groups = 1; next }
  in_groups && /^[[:space:]]+- / {
    sub(/^[[:space:]]+- /, "")
    print
    next
  }
  in_groups { exit }
' "${dex_manifest}" | sort -u > "${tmp_dir}/alice-groups"

sed -n 's/.*THVGroup::"\([^"]*\)".*/\1/p' "${policies}" | sort -u |
  while IFS= read -r group; do
    if ! grep -Fx -- "${group}" "${tmp_dir}/alice-groups" >/dev/null; then
      echo "Alice does not belong to analyzed group '${group}'" >&2
      exit 1
    fi
  done

"${root_dir}/scripts/generate-exfiltration-transitions.sh" "${environments}" "${transitions}"

woodpecker="${CEDAR_WOODPECKER:-}"
if [[ -z "${woodpecker}" ]]; then
  local_woodpecker="${root_dir}/.state/bin/cedar-woodpecker"
  if [[ -x "${local_woodpecker}" ]]; then
    woodpecker="${local_woodpecker}"
  fi
fi
if [[ -z "${woodpecker}" ]]; then
  woodpecker="$(command -v cedar-woodpecker || true)"
fi
if [[ -z "${woodpecker}" || ! -x "${woodpecker}" ]]; then
  echo "repo-local cedar-woodpecker was not found; follow the installation steps in DEMO.md" >&2
  exit 1
fi
if ! "${woodpecker}" --version | grep -Fq 'cedar-woodpecker 0.1.0'; then
  echo "cedar-woodpecker 0.1.0 is required" >&2
  exit 1
fi

if [[ -z "${CVC5:-}" ]]; then
  if [[ -x "${root_dir}/.state/cvc5/bin/cvc5" ]]; then
    CVC5="${root_dir}/.state/cvc5/bin/cvc5"
  elif command -v cvc5 >/dev/null 2>&1; then
    CVC5="$(command -v cvc5)"
  else
    echo "cvc5 was not found; follow the installation steps in DEMO.md" >&2
    exit 1
  fi
fi
export CVC5
if ! "${CVC5}" --version | grep -Fq 'cvc5 version 1.3.1'; then
  echo "cvc5 1.3.1 is required" >&2
  exit 1
fi

read_count="$(jq '.internal_data_readers | length' "${environments}")"
write_count="$(jq '.public_internet_writers | length' "${environments}")"
expected_count=$((read_count * write_count))
generated_count="$(jq '.transitions | length' "${transitions}")"
if [[ "${generated_count}" -ne "${expected_count}" ]]; then
  echo "expected ${expected_count} generated transitions, got ${generated_count}" >&2
  exit 1
fi

"${woodpecker}" escalate \
  --schema "${schema}" \
  --policies "${policies}" \
  --transitions "${transitions}" \
  --json > "${results}"

jq --exit-status \
  --slurpfile generated "${transitions}" '
    ([.[].transition] | unique | sort) == ([$generated[0].transitions[].name] | sort) and
    all(.[];
      .sound == true and
      (.policy | contains("Action::\"exfiltrate_data\"")) and
      (.policy | contains("THVGroup::\"engineering\"")) and
      (.policy | contains("THVGroup::\"support\""))
    )
  ' "${results}" >/dev/null

path_count="$(jq 'length' "${results}")"
printf '%s internal-data reader x %s public-internet writers = %s transitions, %s satisfiable paths\n' \
  "${read_count}" "${write_count}" "${expected_count}" "${path_count}"
jq -r '
  .[] |
  "\nRISK: \(.transition)\n\(.policy)"
' "${results}"
