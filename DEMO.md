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
- GoFetch: `1.0.5`

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
commands. Show the audience the current context and the cluster baseline:

```bash
kubectl config current-context
kubectl cluster-info
kubectl get nodes
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

## 4. Install Dex

Create an isolated namespace, then install the local Dex identity provider:

```bash
kubectl apply -f manifests/00-namespace.yaml
kubectl apply -f manifests/10-dex.yaml
kubectl -n toolhive-demo rollout status deployment/dex --timeout=3m
```

Dex has two static demo identities. Both use the password `password`:

| User | Groups |
| --- | --- |
| `alice@example.com` | `toolhive-users`, `engineering` |
| `bob@example.com` | `toolhive-users`, `finance` |

The shared `toolhive-users` group is what the first Cedar policy will permit.

## 5. Apply the Cedar policy

Show the policy before applying it:

```bash
cat policies/all-dex-users.yaml
kubectl apply -f policies/all-dex-users.yaml
```

Its Cedar statement permits principals in the `toolhive-users` group to invoke
only the GoFetch `fetch` tool:

```cedar
permit(
  principal in THVGroup::"toolhive-users",
  action == Action::"call_tool",
  resource == Tool::"fetch"
);
```

## 6. Create the MCP resources

Inspect the resources if you want to introduce them before applying them:

```bash
cat manifests/20-toolhive.yaml
kubectl apply -f manifests/20-toolhive.yaml
```

That manifest creates four things:

1. An `MCPOIDCConfig` that trusts Dex.
2. An `MCPGroup` for the demo backends.
3. An `MCPServer` running the GoFetch image.
4. A `VirtualMCPServer` that aggregates the group and enforces OIDC and Cedar.

Wait for each resource so any startup problem is obvious:

```bash
kubectl -n toolhive-demo wait \
  --for=condition=Valid mcpoidcconfig/dex --timeout=2m
kubectl -n toolhive-demo wait \
  --for=condition=Valid mcpauthzconfig/fetch-access --timeout=2m
kubectl -n toolhive-demo wait \
  --for=jsonpath='{.status.phase}'=Ready mcpserver/gofetch --timeout=5m
kubectl -n toolhive-demo wait \
  --for=jsonpath='{.status.phase}'=Ready mcpgroup/demo-backends --timeout=5m
kubectl -n toolhive-demo wait \
  --for=jsonpath='{.status.phase}'=Ready virtualmcpserver/cedar-demo --timeout=5m
```

Finally, show what the operator created:

```bash
task status
```

## 7. Demonstrate authorization

Both users belong to `toolhive-users`, so both can discover and invoke `fetch`:

```bash
task demo USER=alice@example.com
task demo USER=bob@example.com
```

Now restrict access to the `engineering` group:

```bash
cat policies/engineering-only.yaml
task policy-engineering
task demo USER=alice@example.com
task demo USER=bob@example.com
```

Alice is allowed. Bob sees an empty tool list and gets HTTP 403 when he tries to
call `fetch`.

Next, authorize a specific identity instead of a group:

```bash
cat policies/alice-only.yaml
task policy-alice
task demo USER=alice@example.com
task demo USER=bob@example.com
```

Restore the all-Dex-users policy when you finish:

```bash
task policy-all
```

The Task helpers automate token acquisition, port-forwarding, MCP session setup,
and waiting for policy reconciliation. The cluster and every installed resource
remain explicit steps above because those are part of the story you are telling.

## Cleanup

Delete only this named Kind cluster, then remove its saved kubeconfig:

```bash
kind delete cluster --name toolhive-cedar-demo
rm -f .state/kubeconfig
```

## Recovery commands

If something does not become ready on stage, these provide a quick view of the
current state:

```bash
task status
task logs
kubectl -n toolhive-system get events --sort-by=.lastTimestamp
kubectl -n toolhive-demo get events --sort-by=.lastTimestamp
```
