# ToolHive + Dex + Cedar demo

This standalone conference demo creates a disposable Kind cluster containing:

- ToolHive Operator and CRDs installed from the official OCI Helm charts
- Dex with one local user and group claims
- MKP as a read-only Kubernetes MCP backend with intentionally overprivileged
  cluster access
- The official GitHub MCP server with its token loaded from a local file
- A Virtual MCP server protected by OIDC and Cedar

The vMCP exposes MKP's `list_resources` tool plus GitHub's
`issue_read` and `add_issue_comment` tools. Cedar grants the MKP tool to
`THVGroup::"engineering"` and the GitHub tools to
`THVGroup::"support"`. Alice belongs to both groups, creating an indirect
Kubernetes-to-GitHub exfiltration path.

The repository includes a demo of `cedar-woodpecker`, an experimental
privilege-escalation analysis tool. It models internal-data readers and
public-internet writers as two maintained lists, generates their Cartesian
product as transition functions, and synthesizes the implicit
`exfiltrate_data` permission. See
[`analysis/README.md`](analysis/README.md).

The [`policies/20-combined-access.cedar`](policies/20-combined-access.cedar)
file is the source of truth for the vulnerable Cedar policy set. It contains a
deliberate bug that is fixed in
[`policies/20-combined-access-fixed.cedar`](policies/20-combined-access-fixed.cedar).
The ToolHive YAML manifests, transition JSON, and synthesized implicit Cedar
policy are generated and checked for drift.

## Pinned releases

These versions were pinned when this repository was prepared on 24 September
2026:

| Component | Version |
| --- | --- |
| ToolHive operator and CRD charts | `0.51.0` |
| Dex | `v2.45.1` |
| MKP | `v0.4.3` |
| GitHub MCP Server | `v1.12.2` |
| Cedar Woodpecker | `7015a6fa38b4d48a748443d1aa85f5b741f51200` |
| cvc5 | `1.3.1` |

The versions are pinned so the talk remains reproducible. Update the chart
versions in `DEMO.md` and the image tags in `manifests/` when you intentionally
want to move to newer releases.

## Conference walkthrough

Supported hosts are macOS, Linux, and WSL2. Prerequisites: Docker,
[Task](https://taskfile.dev/), Bash, `kind`, `kubectl`, Helm 3.10 or newer,
curl 7.76 or newer, and `jq`. The optional compound-permission analysis also
requires Go 1.23 or newer, Rust 1.89 or newer, Cedar Woodpecker, and cvc5
1.3.1; `DEMO.md` contains pinned installation commands.

```bash
git clone https://github.com/ChrisJBurns/toolhive-cedar-demo.git
cd toolhive-cedar-demo
```

Follow [DEMO.md](DEMO.md) for the command-by-command walkthrough.
`task setup-demo-cluster` runs named setup stages in order, while the runbook
explains what each stage installs and what to show on screen.

The token file is gitignored. To use a file elsewhere, override its location:

```bash
task clean-and-create-github-token-secret GITHUB_TOKEN_FILE=/secure/path/github-token
```

Use a fine-grained token with repository access and read/write Issues
permission. ToolHive creates the `github-token` Kubernetes Secret from the file
and injects it into the GitHub MCP server; the token never enters a manifest.

Alice uses the password `password`:

| User | Dex groups |
| --- | --- |
| `alice@example.com` | `engineering`, `support` |

The `demo` task obtains a Dex JWT, opens temporary port-forwards, initializes an
MCP session, and lists the filtered tools without calling them.

## Talk sequence

Create the complete cluster with the vulnerable combined policy, then show
Alice's effective permissions:

```bash
task setup-demo-cluster
task demo USER=alice@example.com
```

The combined policy grants `list_resources` through `engineering` and grants
`issue_read` and `add_issue_comment` through `support`. The resulting
internal-data reader and external writer enable the exfiltration demonstrated
in `DEMO.md`.

Analyze both policy sets, then apply the bounded version:

```bash
task analyze-exfiltration
task policy-fixed
```

The vulnerable policy produces one implicit exfiltration path. The fixed
policy produces none because the support role has an explicit
ceiling, while engineering-only principals retain `list_resources`.

MKP's `get_resource` and the GitHub server's other tools are hidden by the vMCP
allow-list. This keeps tool filtering separate from Cedar's identity-based
authorization decision.

## Connect another MCP client

Keep the port-forwards running in a separate terminal:

```bash
task forward
```

The MCP endpoint is `http://127.0.0.1:4483/mcp`. Generate a bearer token with:

```bash
task token USER=alice@example.com
```

Configure the client to send that value as `Authorization: Bearer <token>`.

## Useful commands

```bash
task status    # Resource phases, pods, and services
task versions  # Installed charts and container images
task github-secret GITHUB_TOKEN_FILE=/secure/path/github-token  # Rotate token
task logs      # Operator, vMCP, and Dex logs
task ready     # Wait for every resource
task analyze-exfiltration  # Compare vulnerable and fixed Cedar policies
task generate-analysis     # Regenerate policy and analysis artifacts
task test-analysis         # Test the Go generators
task cleanup   # Delete the demo Kind cluster and saved kubeconfig
```

This is deliberately a local-only demo. It grants MKP the cluster-wide built-in
`cluster-admin` role and enables HTTP OIDC, the OAuth password grant, static
users, and a known client secret for deterministic conference use. MKP can read
the Kubernetes Secret containing the live GitHub token. Use a short-lived,
fine-grained token, protect and remove its local file when it is no longer
needed, and delete the demo cluster promptly. Do not reuse these settings in
production.
