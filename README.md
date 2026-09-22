# ToolHive + Dex + Cedar demo

This standalone conference demo creates a disposable Kind cluster containing:

- ToolHive Operator and CRDs installed from the official OCI Helm charts
- Dex with two local users and group claims
- MKP as a read-only Kubernetes MCP backend
- A Virtual MCP server protected by OIDC and Cedar

The vMCP exposes only MKP's `list_resources` tool. The Cedar policy permits that
tool for `THVGroup::"engineering"`; membership of the broader
`toolhive-users` group grants no access.

## Pinned releases

These were the latest releases when this repository was prepared on
22 September 2026:

| Component | Version |
| --- | --- |
| ToolHive operator and CRD charts | `0.51.0` |
| Dex | `v2.45.1` |
| MKP | `v0.4.3` |

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
task up
task demo USER=alice@example.com
task demo USER=bob@example.com
```

`task up` creates the `toolhive-cedar-demo` Kind cluster, installs the official
ToolHive charts from GHCR, applies all demo resources, and waits for them to
become ready.

For the command-by-command stage setup, follow [DEMO.md](DEMO.md).

Both users have the password `password`:

| User | Dex groups |
| --- | --- |
| `alice@example.com` | `toolhive-users`, `engineering` |
| `bob@example.com` | `toolhive-users`, `finance` |

The `demo` task obtains a Dex JWT, opens temporary port-forwards, initializes an
MCP session, lists the filtered tools, and calls `list_resources` for pods in
the demo namespace.

## Talk sequence

Apply the engineering-only policy to restore the intended state at any time:

```bash
task policy-engineering
```

Alice belongs to both `toolhive-users` and `engineering`. She sees only
`list_resources` and can call it:

```bash
task demo USER=alice@example.com
```

Bob belongs to `toolhive-users` but not `engineering`. Cedar removes the tool
from his list and returns HTTP 403 when he attempts the call:

```bash
task demo USER=bob@example.com
```

MKP also provides `get_resource`, but the vMCP allow-list hides it. This keeps
tool filtering separate from Cedar's identity-based authorization decision.

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
task down      # Delete the demo-owned Kind cluster
```

This is deliberately a local-only demo. It grants MKP the cluster-wide built-in
`view` role and enables HTTP OIDC, the OAuth password grant, static users, and a
known client secret for deterministic conference use. Do not reuse these
settings in production.
