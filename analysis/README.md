# Cedar Woodpecker analysis

This analysis asks whether explicit ToolHive permissions compose into an
implicit ability to exfiltrate data.

[`policies/toolhive.cedarschema`](../policies/toolhive.cedarschema) models the
relevant subset of ToolHive's Cedar runtime mapping:

- JWT subjects become `Client` principals.
- JWT groups become `THVGroup` parents of the principal.
- MCP tool calls use `Action::"call_tool"` and `Tool::<tool-name>`.

The schema adds `Action::"exfiltrate_data"` on an `Exfiltration` resource as an
analysis-only target. ToolHive neither executes that action nor loads the
schema. Cedar Woodpecker uses it to synthesize and type-check the implicit
permission.

## Sources and generated artifacts

The native files in [`policies/`](../policies/) are authoritative. The Go
generators produce and verify these derived files:

- `policies/demo/*.yaml` and `policies/demo-fixed/*.yaml` for ToolHive.
- [`exfiltration-transitions.json`](exfiltration-transitions.json) for Cedar
  Woodpecker.
- [`policies/implicit/with-implicit-permissions.cedar`](../policies/implicit/with-implicit-permissions.cedar)
  for the synthesized finding.

[`request-environments.json`](request-environments.json) maintains one list of
exact tool requests classified as internal-data reads and another classified as
public-internet writes. With `n` readers and `m` writers, the generator produces
`n*m` transition functions.

All three MCP tools occupy the typed request environment
`(Client, Action::"call_tool", Tool)`. Values such as
`Tool::"list_resources"` are concrete entity UIDs, not separate typed request
environments. The global transition `when` clause pins each source request to
its exact Tool UID. Cedar Woodpecker does not yet support a separate `when`
clause on each source.

The target needs only the `exfiltrate_data` action and `Exfiltration` resource
type. A concrete target entity would add no information because this demo has a
single exfiltration destination.

## Vulnerable and fixed results

The current lists produce one combination:

1. `list_resources` + `add_issue_comment` -> `exfiltrate_data`

`issue_read` is exposed by the live vMCP filter and policy, but it is read-only
and therefore is not classified as a public-internet writer.

The vulnerable policy yields one sound implicit policy requiring both
`THVGroup::"engineering"` and `THVGroup::"support"`. The fixed policy yields no
path because its `forbid` bounds the support role to the two GitHub issue tools.
Cedar Woodpecker's explicit-permission cubes
also verify that the fix retains engineering-only `list_resources` and both
intended GitHub permissions.

After installing Cedar Woodpecker and cvc5, run:

```bash
task analyse-policy
```

To analyse only the fixed policy and show that it has zero exfiltration paths,
run:

```bash
task analyse-policy-fixed
```

To intentionally refresh every generated artifact, run:

```bash
task generate-analysis
```

CI runs the check variants and the Go unit tests so source and generated output
cannot drift. The Cedar Woodpecker and cvc5 binaries remain beneath the
gitignored `.state/` directory; generated analysis evidence is committed.
