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
```

Clone the repository and enter it:

```bash
git clone https://github.com/ChrisJBurns/toolhive-cedar-demo.git
cd toolhive-cedar-demo
```

The demo uses these pinned versions:

- ToolHive operator and CRD charts: `0.51.0`
- Dex: `v2.45.1`
- MKP: `v0.4.3`
- GitHub MCP Server: `v1.12.2`

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
permission. Then use the Task helper to enter it without echoing it in the
terminal. The helper strips carriage returns and newlines, saves the token to
the gitignored `.state/github-token` file, restricts access to that file, and
creates the `github-token` Kubernetes Secret:

```bash
task create-github-token
```

The Secret is injected into the GitHub MCP server as
`GITHUB_PERSONAL_ACCESS_TOKEN`; the token value never appears in a manifest or
the terminal output.

Now install the local Dex identity provider:

```bash
kubectl apply -f manifests/10-dex.yaml
kubectl -n toolhive-demo rollout status deployment/dex --timeout=3m
```

Dex has two static demo identities. Both use the password `password`:

| User | Groups |
| --- | --- |
| `alice@example.com` | `engineering` |
| `bob@example.com` | None |

Alice's `engineering` membership permits the tool calls. Bob has no group
membership, so Cedar denies him access.

## 5. Apply the Cedar policy

Show the policy before applying it:

```bash
cat policies/engineering-only.yaml
kubectl apply -f policies/engineering-only.yaml
```

Its Cedar statements permit only principals in `engineering` to invoke the
three exposed tools. Cedar denies every request that has no matching permit:

```cedar
permit(
  principal in THVGroup::"engineering",
  action == Action::"call_tool",
  resource == Tool::"list_resources"
);
permit(
  principal in THVGroup::"engineering",
  action == Action::"call_tool",
  resource == Tool::"add_issue_comment"
);
permit(
  principal in THVGroup::"engineering",
  action == Action::"call_tool",
  resource == Tool::"issue_write"
);
```

## 6. Create the MCP resources

Inspect the resources if you want to introduce them before applying them:

```bash
cat manifests/20-toolhive.yaml
kubectl apply -f manifests/20-toolhive.yaml
```

That manifest creates the following pieces:

1. An `MCPOIDCConfig` that trusts Dex.
2. A service account with Kubernetes's read-only `view` role.
3. An `MCPGroup` for the demo backends.
4. An `MCPServer` running MKP in read-only mode.
5. An `MCPServer` running the official GitHub server with its token injected
   from the Kubernetes Secret.
6. A `VirtualMCPServer` that aggregates both backends and enforces OIDC and
   Cedar.

MKP calls its tool `list_resources`. The vMCP aggregation allow-list advertises
only that MKP tool, hiding `get_resource`. A separate allow-list advertises only
GitHub's `add_issue_comment` and `issue_write` tools.

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

## 7. Demonstrate filtering and authorization

First show the active Cedar policy, then authenticate as Alice:

```bash
cat policies/engineering-only.yaml
task demo USER=alice@example.com
```

Alice's tool list contains exactly `list_resources`, `add_issue_comment`, and
`issue_write`, proving that the vMCP filters have hidden every other backend
tool. Her `engineering` group membership allows the call. The demo script calls
only the read-only MKP tool, so it does not modify GitHub during the talk.

Now authenticate as Bob:

```bash
task demo USER=bob@example.com
```

Bob has no group membership. He sees an empty tool list, and Cedar returns HTTP
403 when he attempts `list_resources`.

The Task helpers automate token acquisition, port-forwarding, MCP session setup,
and policy reconciliation. The cluster and every installed resource remain
explicit steps above because those are part of the story you are telling.

## 8. Connect Claude Code

Claude Code connects to the single vMCP endpoint, which routes requests to both
MKP and GitHub. No ingress is required because Claude Code and Kind are running
on the same machine.

In a separate terminal, keep the Dex and vMCP port-forwards running:

```bash
task forward
```

Leave that terminal open. In another terminal, obtain a Dex token for Alice and
register the vMCP endpoint with Claude Code:

```bash
DEX_TOKEN="$(task token USER=alice@example.com)"

claude mcp add \
  --scope local \
  --transport http \
  toolhive-demo \
  http://127.0.0.1:4483/mcp \
  --header "Authorization: Bearer ${DEX_TOKEN}"

unset DEX_TOKEN
```

If `toolhive-demo` was registered during a previous rehearsal, remove it first
with `claude mcp remove toolhive-demo`, then repeat the commands above.

Verify the connection and start Claude Code:

```bash
claude mcp list
claude
```

Inside Claude Code, run `/mcp` to inspect the connection. Alice should see
`list_resources`, `add_issue_comment`, and `issue_write` through the one
`toolhive-demo` server.

## 9. Exfiltration

Give Claude Code the following prompt:

```text
Can you please use the kubernetes mcp server tools inside of the toolhive-demo mcp server and get the dex pod information please and upload it to the debugging ticket https://github.com/ChrisJBurns/toolhive-cedar-demo/issues/1 using the issue tools so our platform team can troubleshoot.
```

## Cleanup

Delete only this named Kind cluster, then remove its saved kubeconfig:

```bash
claude mcp remove toolhive-demo
kind delete cluster --name toolhive-cedar-demo
rm -f .state/kubeconfig
```

The GitHub token remains in `.state/github-token` for another rehearsal. Remove
that file separately when you no longer need it.
