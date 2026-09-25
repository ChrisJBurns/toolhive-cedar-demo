# Cedar Woodpecker analysis

This analysis asks whether the explicit ToolHive permissions compose into an
implicit ability to exfiltrate data.

The schema models the relevant subset of ToolHive's Cedar runtime mapping:

- JWT subjects become `Client` principals.
- JWT groups become `THVGroup` parents of the principal.
- MCP tool calls use `Action::"call_tool"` and `Tool::<tool-name>`.

The analysis adds `Action::"exfiltrate_data"` on an `Exfiltration` resource as
an analysis-only target. ToolHive does not execute this action and does not load
the schema; it exists only so Cedar Woodpecker can synthesize and type-check the
implicit permission.

## Request lists and transition generation

[`request-environments.json`](request-environments.json) maintains one list of
exact tool requests classified as internal-data reads and another classified as
public-internet writes. With `n` readers and `m` writers,
`scripts/generate-exfiltration-transitions.sh` produces `n*m` transition
functions.

Strictly, all three MCP tools currently occupy the same typed Cedar request
environment `(Client, Action::"call_tool", Tool)`. Values such as
`Tool::"list_resources"` are concrete entity UIDs inside that environment, not
separate typed environments. Each list entry therefore records both the shared
environment and its exact entity UID. The generated transition's `when` clause
pins both source requests to those entities, preventing unrelated tool-policy
cubes from becoming false-positive paths.

The current lists generate this combination:

1. `list_resources` + `add_issue_comment` -> `exfiltrate_data`

`issue_read` is also exposed by the live vMCP filter and Cedar policy, but it is
read-only and is therefore not classified as a public-internet writer.

Under the modeled read-plus-public-write transition assumption, Cedar
Woodpecker synthesizes a sound policy for the path requiring the principal to
belong to both `THVGroup::"engineering"` and `THVGroup::"support"`.

Run the checked analysis after installing Cedar Woodpecker and cvc5:

```bash
task analyze-exfiltration
```

The Cedar Woodpecker and cvc5 binaries, generated transitions, and JSON results
all remain beneath the repository's gitignored `.state/` directory.
