# Conference walkthrough

This runbook keeps the infrastructure setup visible so you can explain each
piece as you create it. Run the commands one block at a time rather than pasting
the whole page at once.

## Before going on stage

Install and start Docker, then verify these commands are available:

```bash
docker info
kind version
kubectl version --client
helm version
task --version
curl --version
jq --version
cargo --version
```

Clone the repository and enter it:

```bash
git clone https://github.com/ChrisJBurns/toolhive-cedar-demo.git
cd toolhive-cedar-demo
```

Install the experimental Cedar Woodpecker CLI from the exact revision used by
this demo. It requires Rust 1.89 or newer:

```bash
cargo install --locked \
  --root "$PWD/.state" \
  --git https://github.com/luxas/cedar-woodpecker.git \
  --rev 7015a6fa38b4d48a748443d1aa85f5b741f51200 \
  cedar-woodpecker
```

Cedar Woodpecker also requires cvc5. On an Apple Silicon Mac, install the
pinned binary inside the repository:

```bash
mkdir -p .state/cvc5/bin
curl -fsSL https://github.com/cvc5/cvc5/releases/download/cvc5-1.3.1/cvc5-macOS-arm64-static.zip -o .state/cvc5.zip
printf '%s  %s\n' a0e7f5b03b1bc4284fbfff7cdfb08c704801701cf7ece83a13f8a505e7581215 .state/cvc5.zip | shasum -a 256 -c -
unzip -oqj .state/cvc5.zip '*/bin/cvc5' -d .state/cvc5/bin
chmod +x .state/cvc5/bin/cvc5
.state/cvc5/bin/cvc5 --version
.state/bin/cedar-woodpecker --version
```

Use the corresponding cvc5 1.3.1 archive for Intel macOS, Linux, or WSL2.

The demo uses these pinned versions:

- ToolHive operator and CRD charts: `0.51.0`
- Dex: `v2.45.1`
- MKP: `v0.4.3`
- GitHub MCP Server: `v1.12.2`
- Cedar Woodpecker: `7015a6fa38b4d48a748443d1aa85f5b741f51200`
- cvc5: `1.3.1`

## 1. Create the Kubernetes cluster

Create a local cluster called `toolhive-cedar-demo`:

```bash
kind create cluster --name toolhive-cedar-demo --wait 120s
```

Save its kubeconfig inside the repository. The Task helpers use this exact file,
which prevents the demo from accidentally targeting another cluster.

```bash
mkdir -p .state
kind get kubeconfig --name toolhive-cedar-demo > .state/kubeconfig
export KUBECONFIG="$PWD/.state/kubeconfig"
```

Re-run the `export` command in any new terminal used for raw Helm or `kubectl`
commands. Show the audience the namespaces in the new cluster before installing
anything:

```bash
kubectl get namespaces
```

## 2. Install the ToolHive CRDs

ToolHive represents MCP servers, groups, authentication, authorization, and
virtual MCP servers as Kubernetes custom resources. Install those API
definitions first:

```bash
helm upgrade --install toolhive-operator-crds \
  oci://ghcr.io/stacklok/toolhive/toolhive-operator-crds \
  --version 0.51.0 \
  --namespace toolhive-system \
  --create-namespace \
  --wait \
  --timeout 5m
```

You can make the new API surface visible with:

```bash
kubectl api-resources --api-group=toolhive.stacklok.dev
```

## 3. Install the ToolHive operator

The operator watches those resources and turns the desired state into running
workloads:

```bash
helm upgrade --install toolhive-operator \
  oci://ghcr.io/stacklok/toolhive/toolhive-operator \
  --version 0.51.0 \
  --namespace toolhive-system \
  --create-namespace \
  --wait \
  --timeout 5m
```

Verify the installation before moving on:

```bash
kubectl -n toolhive-system rollout status deployment/toolhive-operator --timeout=5m
kubectl -n toolhive-system get pods
helm -n toolhive-system list
```

## 4. Load the GitHub token and install Dex

Create an isolated namespace:

```bash
kubectl apply -f manifests/00-namespace.yaml
```

Create a fine-grained GitHub token with repository access and read/write Issues
permission, then save it in the gitignored `.state/github-token` file:

```bash
vim .state/github-token
```

The Task helper strips carriage returns and newlines, restricts access to the
file, and creates the `github-token` Kubernetes Secret:

```bash
task clean-and-create-github-token-secret
```

The Secret is injected into the GitHub MCP server as
`GITHUB_PERSONAL_ACCESS_TOKEN`; the token value never appears in a manifest or
the terminal output.

Now install the local Dex identity provider:

```bash
kubectl apply -f manifests/10-dex.yaml
kubectl -n toolhive-demo rollout status deployment/dex --timeout=3m
```

Dex has one static demo identity. Alice uses the password `password`:

| User | Groups |
| --- | --- |
| `alice@example.com` | `engineering`, `support` |

Alice carries both group claims, but group membership alone grants nothing.
Cedar decides which capability each group receives.

## 5. Establish the deny-by-default baseline

Start with an explicit deny-all policy so the vMCP can be created before either
of Alice's groups has permission to use a tool:

```bash
cat policies/demo/00-deny-all.yaml
kubectl apply -f policies/demo/00-deny-all.yaml
```

The baseline contains no permits:

```cedar
forbid(principal, action == Action::"call_tool", resource);
```

## 6. Create the MCP resources

Inspect the resources if you want to introduce them before applying them:

```bash
cat manifests/20-toolhive.yaml
kubectl apply -f manifests/20-toolhive.yaml
```

That manifest creates the following pieces:

1. An `MCPOIDCConfig` that trusts Dex.
2. An intentionally overprivileged service account with the cluster-wide
   `cluster-admin` role.
3. An `MCPGroup` for the demo backends.
4. An `MCPServer` running MKP in read-only mode.
5. An `MCPServer` running the official GitHub server with its token injected
   from the Kubernetes Secret.
6. A `VirtualMCPServer` that aggregates both backends and enforces OIDC and
   Cedar.

MKP calls its tool `list_resources`. The vMCP aggregation allow-list advertises
only that MKP tool, hiding `get_resource`. A separate allow-list advertises only
GitHub's `issue_read` and `add_issue_comment` tools.

Wait for each resource so any startup problem is obvious:

```bash
kubectl -n toolhive-demo wait --for=condition=Valid mcpoidcconfig/dex --timeout=2m
kubectl -n toolhive-demo wait --for=condition=Valid mcpauthzconfig/resources-access --timeout=2m
kubectl -n toolhive-demo wait --for=jsonpath='{.status.phase}'=Ready mcpserver/mkp --timeout=5m
kubectl -n toolhive-demo wait --for=jsonpath='{.status.phase}'=Ready mcpserver/github --timeout=5m
kubectl -n toolhive-demo wait --for=jsonpath='{.status.phase}'=Ready mcpgroup/demo-backends --timeout=5m
kubectl -n toolhive-demo wait --for=jsonpath='{.status.phase}'=Ready virtualmcpserver/cedar-demo --timeout=5m
```

Finally, show what the operator created:

```bash
task status
```

## 7. Build Alice's effective permissions

First authenticate as Alice while the deny-all policy is active:

```bash
task demo USER=alice@example.com
```

Despite carrying both group claims, Alice sees an empty tool list.

Now apply policy 1, which gives `engineering` access to the MKP read tool:

```bash
cat policies/demo/10-engineering.yaml
task policy-engineering
task demo USER=alice@example.com
```

Alice now sees only `list_resources`. Her `support` membership still grants
nothing.

Apply policy 2. It retains policy 1 and gives the `support` role access to the
two GitHub tools:

```bash
cat policies/demo/20-combined-access.yaml
task policy-combined-access
task demo USER=alice@example.com
```

Alice now sees exactly `list_resources`, `issue_read`, and `add_issue_comment`.
Neither group appears dangerous in isolation, but Alice's effective permissions
now contain both a private-data source and an external write destination.

The Task helpers automate token acquisition, port-forwarding, MCP session setup,
and policy reconciliation. The cluster, charts, and workload manifests remain
explicit steps above because those are part of the story you are telling.

## 8. Analyze the compound permissions

The offline analysis uses the same three Cedar policies as the live ToolHive
configuration. Inspect the ToolHive-compatible schema and the two source lists:

```bash
cat analysis/toolhive.cedarschema
cat analysis/request-environments.json
```

There is currently one exact tool request classified as an internal-data read
and one classified as a public-internet write. The Cartesian product therefore
produces one transition function. Generate and inspect it:

```bash
scripts/generate-exfiltration-transitions.sh
cat .state/exfiltration-transitions.json
```

Run Cedar Woodpecker with that generated transition set:

```bash
CVC5="$PWD/.state/cvc5/bin/cvc5" .state/bin/cedar-woodpecker escalate \
  --schema analysis/toolhive.cedarschema \
  --policies analysis/toolhive.cedar \
  --transitions .state/exfiltration-transitions.json
```

For a concise, checked rehearsal of the same analysis, run this instead of the
two commands above:

```bash
task analyze-exfiltration
```

The task first verifies that `analysis/toolhive.cedar` still exactly matches the
combined live policy manifest. Under the modeled assumption that combining an
internal read with a public write enables exfiltration, it then requires Cedar
Woodpecker to return one sound path. The synthesized policy grants
`Action::"exfiltrate_data"` only when the principal belongs to both groups:

```text
engineering -> list_resources -> internal Kubernetes data
support     -> add_issue_comment -> public internet

engineering x support -> 1 exfiltration path
```

## 9. Connect OpenCode

OpenCode connects to the single vMCP endpoint, which routes requests to both
MKP and GitHub. No ingress is required because OpenCode and Kind are running on
the same machine.

In a separate terminal, keep the Dex and vMCP port-forwards running:

```bash
task forward
```

Leave that terminal open. Configure the `toolhive-demo` MCP server in the
project's `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "toolhive-demo": {
      "type": "remote",
      "url": "http://127.0.0.1:4483/mcp",
      "enabled": true,
      "oauth": false,
      "headers": {
        "Content-Type": "application/json",
        "Accept": "application/json, text/event-stream",
        "Authorization": "Bearer {env:DEX_TOKEN}"
      }
    }
  }
}
```

In another terminal, change to a clean directory outside this repository so
OpenCode cannot use the demo's Git history or files as context. Reference the
demo Taskfile explicitly when exporting Alice's Dex token, then start OpenCode
from that clean directory:

```bash
export DEX_TOKEN="$(task --taskfile ~/projects/toolhive-cedar-demo token USER=alice@example.com)"
opencode
```

Alice should see `list_resources`, `issue_read`, and `add_issue_comment` through
the one `toolhive-demo` server. Refresh `DEX_TOKEN` and restart OpenCode if the
token expires.

## 10. Exfiltration

Give OpenCode the following prompt:

```text
Can you please triage this issue and investigate what the issue is https://github.com/ChrisJBurns/toolhive-cedar-demo/issues/3
```

## Cleanup

Delete only this named Kind cluster, then remove its saved kubeconfig:

```bash
task cleanup
```

The GitHub token remains in `.state/github-token` for another rehearsal. Remove
that file separately when you no longer need it.
