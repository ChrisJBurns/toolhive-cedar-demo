# ToolHive + Dex + Cedar demo

This standalone conference demo walks through a disposable Kind cluster
containing:

- ToolHive Operator and CRDs installed from the official OCI Helm charts
- Dex with two local users and group claims
- GoFetch as the MCP backend
- A Virtual MCP server protected by OIDC and Cedar

The default Cedar policy permits the `fetch` tool to members of
`THVGroup::"toolhive-users"`. Every user configured in this Dex instance belongs
to that group.

## Pinned releases

These were the latest releases when this repository was prepared on
22 September 2026:

| Component | Version |
| --- | --- |
| ToolHive operator and CRD charts | `0.51.0` |
| Dex | `v2.45.1` |
| GoFetch | `v1.0.5` |

The versions are pinned so the talk remains reproducible. Update the chart
versions in `DEMO.md` and the image tags in `manifests/` when you intentionally
want to move to newer releases.

## Run the demo

Supported hosts are macOS, Linux, and WSL2. Prerequisites: Docker,
[Task](https://taskfile.dev/), Bash, `kind`, `kubectl`, Helm 3.10 or newer,
curl 7.76 or newer, and `jq`.

Follow [DEMO.md](DEMO.md) for the complete, copy-and-paste conference runbook.
Cluster creation, Helm installation, and resource creation are intentionally
shown as individual commands so the setup remains visible while presenting.

Both users have the password `password`:

| User | Dex groups |
| --- | --- |
| `alice@example.com` | `toolhive-users`, `engineering` |
| `bob@example.com` | `toolhive-users`, `finance` |

The `demo` task obtains a Dex JWT, opens temporary port-forwards, initializes an
MCP session, lists the Cedar-filtered tools, and calls `fetch` against
`https://example.com`.

## Authorization sequence

Start with the default shared-group policy. Both users are allowed:

```bash
task policy-all
task demo USER=alice@example.com
task demo USER=bob@example.com
```

Restrict access to the engineering group. Alice remains allowed; Bob sees an
empty tool list and receives an HTTP 403 when attempting the call:

```bash
task policy-engineering
task demo USER=alice@example.com
task demo USER=bob@example.com
```

Then demonstrate a user-attribute rule:

```bash
task policy-alice
task demo USER=alice@example.com
task demo USER=bob@example.com
```

Restore the requested all-Dex-users state with `task policy-all`.

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
task logs      # Operator, vMCP, and Dex logs
task ready     # Wait for every resource
```

This is deliberately a local-only demo. It enables HTTP OIDC, the OAuth password
grant, static users, and a known client secret for deterministic conference use.
Do not reuse these settings in production.
