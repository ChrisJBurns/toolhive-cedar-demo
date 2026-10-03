// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"

	"go.yaml.in/yaml/v3"
)

var cedarGroupPattern = regexp.MustCompile(`THVGroup::("(?:[^"\\]|\\.)*")`)

type kubernetesDocument struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
	Spec struct {
		Config struct {
			Aggregation struct {
				Tools []struct {
					Filter []string `yaml:"filter"`
				} `yaml:"tools"`
			} `yaml:"aggregation"`
		} `yaml:"config"`
	} `yaml:"spec"`
}

type dexConfiguration struct {
	StaticPasswords []struct {
		Email  string   `yaml:"email"`
		Groups []string `yaml:"groups"`
	} `yaml:"staticPasswords"`
}

// ValidateManifestBindings verifies that analysis classifications and Cedar
// groups still correspond to the live vMCP and Dex manifests.
func ValidateManifestBindings(
	environments RequestEnvironments,
	vulnerableCedar, toolHiveManifest, dexManifest []byte,
) error {
	exposedTools, err := exposedVirtualMCPTools(toolHiveManifest)
	if err != nil {
		return fmt.Errorf("inspect ToolHive manifest: %w", err)
	}
	classified := append(append([]RequestEnvironment{}, environments.InternalDataReaders...), environments.PublicInternetWriters...)
	for _, request := range classified {
		if _, exists := exposedTools[request.Name]; !exists {
			return fmt.Errorf("classified tool %q is not exposed by the VirtualMCPServer", request.Name)
		}
	}

	aliceGroups, err := dexUserGroups(dexManifest, "alice@example.com")
	if err != nil {
		return fmt.Errorf("inspect Dex manifest: %w", err)
	}
	for _, group := range cedarGroups(vulnerableCedar) {
		if _, exists := aliceGroups[group]; !exists {
			return fmt.Errorf("Alice does not belong to Cedar group %q", group)
		}
	}
	return nil
}

func exposedVirtualMCPTools(manifest []byte) (map[string]struct{}, error) {
	documents, err := decodeKubernetesDocuments(manifest)
	if err != nil {
		return nil, err
	}
	tools := make(map[string]struct{})
	found := false
	for _, document := range documents {
		if document.Kind != "VirtualMCPServer" {
			continue
		}
		found = true
		for _, toolSet := range document.Spec.Config.Aggregation.Tools {
			for _, tool := range toolSet.Filter {
				tools[tool] = struct{}{}
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("VirtualMCPServer document not found")
	}
	return tools, nil
}

func dexUserGroups(manifest []byte, email string) (map[string]struct{}, error) {
	documents, err := decodeKubernetesDocuments(manifest)
	if err != nil {
		return nil, err
	}
	for _, document := range documents {
		if document.Kind != "ConfigMap" || document.Metadata.Name != "dex-config" {
			continue
		}
		config, exists := document.Data["config.yaml"]
		if !exists {
			return nil, fmt.Errorf("dex-config ConfigMap has no config.yaml")
		}
		var dex dexConfiguration
		if err := yaml.Unmarshal([]byte(config), &dex); err != nil {
			return nil, fmt.Errorf("decode dex-config config.yaml: %w", err)
		}
		for _, user := range dex.StaticPasswords {
			if user.Email != email {
				continue
			}
			groups := make(map[string]struct{}, len(user.Groups))
			for _, group := range user.Groups {
				groups[group] = struct{}{}
			}
			return groups, nil
		}
		return nil, fmt.Errorf("Dex user %q not found", email)
	}
	return nil, fmt.Errorf("dex-config ConfigMap not found")
}

func decodeKubernetesDocuments(manifest []byte) ([]kubernetesDocument, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(manifest))
	var documents []kubernetesDocument
	for {
		var document kubernetesDocument
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode Kubernetes YAML: %w", err)
		}
		if document.Kind != "" {
			documents = append(documents, document)
		}
	}
	return documents, nil
}

func cedarGroups(source []byte) []string {
	unique := make(map[string]struct{})
	for _, match := range cedarGroupPattern.FindAllSubmatch(source, -1) {
		group, err := strconv.Unquote(string(match[1]))
		if err == nil {
			unique[group] = struct{}{}
		}
	}
	groups := make([]string, 0, len(unique))
	for group := range unique {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
