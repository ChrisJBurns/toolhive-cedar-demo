#!/usr/bin/env bash

set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
input="${1:-${root_dir}/analysis/request-environments.json}"
output="${2:-${root_dir}/.state/exfiltration-transitions.json}"

jq --exit-status '
  def nonempty_string: type == "string" and length > 0;
  def valid_request:
    (.name | nonempty_string) and
    (.principal | nonempty_string) and
    (.action | nonempty_string) and
    (.resource_type | nonempty_string) and
    (.resource | nonempty_string);
  . as $config |
  ([$config.internal_data_readers[].name, $config.public_internet_writers[].name] | length) as $name_count |
  ([$config.internal_data_readers[].name, $config.public_internet_writers[].name] | unique | length) as $unique_name_count |
  ($config.internal_data_readers | length > 0) and
  ($config.public_internet_writers | length > 0) and
  all($config.internal_data_readers[]; valid_request) and
  all($config.public_internet_writers[]; valid_request) and
  ($config.target.action | nonempty_string) and
  ($config.target.resource_type | nonempty_string) and
  ($config.target.resource | nonempty_string) and
  ($name_count == $unique_name_count)
' "${input}" >/dev/null

mkdir -p "$(dirname "${output}")"

jq '
  . as $config |
  [
    $config.internal_data_readers[] as $reader |
    $config.public_internet_writers[] as $writer |
    if $reader.principal != $writer.principal then
      error("source principals must have the same type")
    else
      {
        name: "exfiltrate-via-\($reader.name)-and-\($writer.name)",
        principal: [$reader.principal],
        sources: [
          {
            action: $reader.action,
            resource: $reader.resource_type
          },
          {
            action: $writer.action,
            resource: $writer.resource_type
          }
        ],
        target: {
          action: $config.target.action,
          resource: $config.target.resource_type
        },
        when: "context.resource1 == \($reader.resource) && context.resource2 == \($writer.resource) && resource == \($config.target.resource)"
      }
    end
  ] |
  {transitions: .}
' "${input}" > "${output}"
