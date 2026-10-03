// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"strings"
	"testing"
)

func TestValidateManifestBindings(t *testing.T) {
	t.Parallel()
	environments := manifestTestEnvironments()
	cedar := []byte(`permit(principal in THVGroup::"engineering", action, resource);
permit(principal in THVGroup::"support", action, resource);`)
	if err := ValidateManifestBindings(environments, cedar, []byte(toolHiveTestManifest), []byte(dexTestManifest)); err != nil {
		t.Fatalf("ValidateManifestBindings() error = %v", err)
	}
}

func TestValidateManifestBindingsRejectsUnexposedClassifiedTool(t *testing.T) {
	t.Parallel()
	environments := manifestTestEnvironments()
	environments.PublicInternetWriters[0].Name = "issue_write"
	err := ValidateManifestBindings(environments, []byte(`permit(principal in THVGroup::"engineering", action, resource);`),
		[]byte(toolHiveTestManifest), []byte(dexTestManifest))
	if err == nil || !strings.Contains(err.Error(), `tool "issue_write" is not exposed`) {
		t.Fatalf("ValidateManifestBindings() error = %v, want unexposed tool error", err)
	}
}

func TestValidateManifestBindingsRejectsMissingAliceGroup(t *testing.T) {
	t.Parallel()
	err := ValidateManifestBindings(manifestTestEnvironments(),
		[]byte(`permit(principal in THVGroup::"security", action, resource);`),
		[]byte(toolHiveTestManifest), []byte(dexTestManifest))
	if err == nil || !strings.Contains(err.Error(), `Alice does not belong to Cedar group "security"`) {
		t.Fatalf("ValidateManifestBindings() error = %v, want missing Alice group error", err)
	}
}

func manifestTestEnvironments() RequestEnvironments {
	return RequestEnvironments{
		InternalDataReaders:   []RequestEnvironment{{Name: "list_resources"}},
		PublicInternetWriters: []RequestEnvironment{{Name: "add_issue_comment"}},
	}
}

const toolHiveTestManifest = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: unrelated
---
apiVersion: toolhive.stacklok.dev/v1beta1
kind: VirtualMCPServer
metadata:
  name: cedar-demo
spec:
  config:
    aggregation:
      tools:
        - workload: mkp
          filter: [list_resources]
        - workload: github
          filter:
            - add_issue_comment
            - issue_read
`

const dexTestManifest = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: dex-config
data:
  config.yaml: |
    staticPasswords:
      - email: alice@example.com
        groups:
          - engineering
          - support
`
