# ToolHive + Dex + Cedar demo

This standalone conference demo creates a disposable Kind cluster containing:

- ToolHive Operator and CRDs installed from the official OCI Helm charts
- Dex with two local users and group claims
- MKP as a read-only Kubernetes MCP backend
- The official GitHub MCP server with its token loaded from a local file
- A Virtual MCP server protected by OIDC and Cedar

The vMCP exposes MKP's `list_resources` tool plus GitHub's
`add_issue_comment` and `issue_write` tools. The Cedar policy permits those
tools for `THVGroup::"engineering"`. Users outside that group have no access.

## Pinned releases

These were the latest releases when this repository was prepared on
22 September 2026:

| Component | Version |
| --- | --- |
| ToolHive operator and CRD charts | `0.51.0` |
| Dex | `v2.45.1` |
| MKP | `v0.4.3` |
| GitHub MCP Server | `v1.12.2` |

The versions are pinned so the talk remains reproducible. Update
`TOOLHIVE_VERSION` in `Taskfile.yml` and the image tags in `manifests/` when you
intentionally want to move to newer releases.

## Quick start

Supported hosts are macOS, Linux, and WSL2. Prerequisites: Docker,
[Task](https://taskfile.dev/), Bash, `kind`, `kubectl`, Helm 3.10 or newer,
curl 7.76 or newer, and `jq`.

```bash
git clone https://github.com/ChrisJBurns/toolhive-cedar-demo.git
cd toolhive-cedar-demo
task create-github-token
task up
task demo USER=alice@example.com
task demo USER=bob@example.com
```

`task up` creates the `toolhive-cedar-demo` Kind cluster, installs the official
ToolHive charts from GHCR, applies all demo resources, and waits for them to
become ready.

The token file is gitignored. To use a file elsewhere, override its location:

```bash
task up GITHUB_TOKEN_FILE=/secure/path/github-token
```

Use a fine-grained token with repository access and read/write Issues
permission. ToolHive creates the `github-token` Kubernetes Secret from the file
and injects it into the GitHub MCP server; the token never enters a manifest.

For the command-by-command stage setup, follow [DEMO.md](DEMO.md).

Both users have the password `password`:

| User | Dex groups |
| --- | --- |
| `alice@example.com` | `engineering` |
| `bob@example.com` | None |

The `demo` task obtains a Dex JWT, opens temporary port-forwards, initializes an
MCP session, lists the filtered tools, and calls `list_resources` for pods in
the demo namespace.

## Talk sequence

Apply the engineering-only policy to restore the intended state at any time:

```bash
task policy-engineering
```

Alice belongs to `engineering`. She sees exactly `list_resources`,
`add_issue_comment`, and `issue_write`. The demo safely calls only
`list_resources`:

```bash
task demo USER=alice@example.com
```

Bob has no group membership. Cedar removes every tool from his list and returns
HTTP 403 when he attempts the call:

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
