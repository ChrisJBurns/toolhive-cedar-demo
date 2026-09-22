# ToolHive + Dex + Cedar demo

This standalone conference demo creates a disposable Kind cluster containing:

- ToolHive Operator and CRDs installed from the official OCI Helm charts
- Dex with two local users and group claims
- MKP as a read-only Kubernetes MCP backend
- The official GitHub MCP server with its token loaded from a local file
- A Virtual MCP server protected by OIDC and Cedar

The vMCP exposes MKP's `list_resources` tool plus GitHub's
`add_issue_comment` and `issue_write` tools. Cedar grants the MKP tool to
`THVGroup::"cluster-view"` and the GitHub tools to
`THVGroup::"engineering"`. Alice belongs to both groups, creating an indirect
Kubernetes-to-GitHub exfiltration path.

## Pinned releases

These were the latest releases when this repository was prepared on
22 September 2026:

| Component | Version |
| --- | --- |
| ToolHive operator and CRD charts | `0.51.0` |
| Dex | `v2.45.1` |
| MKP | `v0.4.3` |
| GitHub MCP Server | `v1.12.2` |

The versions are pinned so the talk remains reproducible. Update the chart
versions in `DEMO.md` and the image tags in `manifests/` when you intentionally
want to move to newer releases.

## Conference walkthrough

Supported hosts are macOS, Linux, and WSL2. Prerequisites: Docker,
[Task](https://taskfile.dev/), Bash, `kind`, `kubectl`, Helm 3.10 or newer,
curl 7.76 or newer, and `jq`.

```bash
git clone https://github.com/ChrisJBurns/toolhive-cedar-demo.git
cd toolhive-cedar-demo
```

Follow [DEMO.md](DEMO.md) for the command-by-command stage setup. Cluster
creation, chart installation, and resource application remain explicit so the
audience can see each part of the system being assembled.

The token file is gitignored. To use a file elsewhere, override its location:

```bash
task clean-and-create-github-token-secret GITHUB_TOKEN_FILE=/secure/path/github-token
```

Use a fine-grained token with repository access and read/write Issues
permission. ToolHive creates the `github-token` Kubernetes Secret from the file
and injects it into the GitHub MCP server; the token never enters a manifest.

Both users have the password `password`:

| User | Dex groups |
| --- | --- |
| `alice@example.com` | `cluster-view`, `engineering` |
| `bob@example.com` | None |

The `demo` task obtains a Dex JWT, opens temporary port-forwards, initializes an
MCP session, lists the filtered tools, and calls `list_resources` for pods in
the demo namespace.

## Talk sequence

Start with no tool access:

```bash
task policy-deny-all
```

Then build Alice's effective permissions in two stages:

```bash
task policy-cluster-view
task demo USER=alice@example.com

task policy-combined-access
task demo USER=alice@example.com
```

Policy 1 grants `list_resources` through `cluster-view`. Policy 2 retains that
permission and grants `add_issue_comment` and `issue_write` through
`engineering`. The resulting combination enables the exfiltration demonstrated
in `DEMO.md`.

Bob has no group membership. Cedar removes every tool from his list and returns
HTTP 403 when he attempts the read:

```bash
task demo USER=bob@example.com
```

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
task down      # Delete the demo-owned Kind cluster
```

This is deliberately a local-only demo. It grants MKP the cluster-wide built-in
`view` role and enables HTTP OIDC, the OAuth password grant, static users, and a
known client secret for deterministic conference use. Protect and remove the
local GitHub token file when it is no longer needed. Do not reuse these settings
in production.
