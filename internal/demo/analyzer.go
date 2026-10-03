// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	woodpeckerVersion = "cedar-woodpecker 0.1.0"
	cvc5Version       = "cvc5 version 1.3.1"
)

// Analyzer runs cedar-woodpecker with an explicit cvc5 binary.
type Analyzer struct {
	Woodpecker string
	CVC5       string
}

// Escalation is the typed subset of cedar-woodpecker's JSON result used by the demo.
type Escalation struct {
	Transition string `json:"transition"`
	Path       string `json:"path"`
	Policy     string `json:"policy"`
	Sound      bool   `json:"sound"`
	Problem    any    `json:"problem"`
}

// Cube is one satisfiable explicit permission emitted by cedar-woodpecker.
type Cube struct {
	Principal string   `json:"principal"`
	Action    string   `json:"action"`
	Resource  string   `json:"resource"`
	ID        string   `json:"id"`
	Condition string   `json:"condition"`
	Outcomes  []string `json:"outcomes"`
}

// ValidateTools checks the pinned analysis tools before invoking them.
func (analyzer Analyzer) ValidateTools(ctx context.Context) error {
	if err := requireVersion(ctx, analyzer.Woodpecker, woodpeckerVersion); err != nil {
		return fmt.Errorf("validate cedar-woodpecker: %w", err)
	}
	if err := requireVersion(ctx, analyzer.CVC5, cvc5Version); err != nil {
		return fmt.Errorf("validate cvc5: %w", err)
	}
	return nil
}

// Analyze runs cedar-woodpecker against one policy set.
func (analyzer Analyzer) Analyze(ctx context.Context, schema, transitions string, policies []byte) ([]Escalation, error) {
	command := exec.CommandContext(ctx, analyzer.Woodpecker,
		"escalate", "--schema", schema, "--transitions", transitions, "--json")
	command.Stdin = bytes.NewReader(policies)
	command.Env = withEnvironment(os.Environ(), "CVC5", analyzer.CVC5)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("run cedar-woodpecker: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var results []Escalation
	// cedar-woodpecker includes additional diagnostic fields, so retain the
	// fields the demo validates and intentionally tolerate the rest.
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("decode cedar-woodpecker output: %w", err)
	}
	return results, nil
}

// Cubes returns the effective explicit permissions for one policy set.
func (analyzer Analyzer) Cubes(ctx context.Context, schema string, policies []byte) ([]Cube, error) {
	command := exec.CommandContext(ctx, analyzer.Woodpecker, "cubes", "--schema", schema, "--json")
	command.Stdin = bytes.NewReader(policies)
	command.Env = withEnvironment(os.Environ(), "CVC5", analyzer.CVC5)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("run cedar-woodpecker cubes: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var cubes []Cube
	if err := json.Unmarshal(output, &cubes); err != nil {
		return nil, fmt.Errorf("decode cedar-woodpecker cubes output: %w", err)
	}
	return cubes, nil
}

// ValidateVulnerableResults enforces the conference demo's expected finding.
func ValidateVulnerableResults(vulnerable []Escalation, transitionName string) error {
	if len(vulnerable) != 1 {
		return fmt.Errorf("vulnerable policies: expected exactly one path, got %d", len(vulnerable))
	}
	result := vulnerable[0]
	if result.Transition != transitionName {
		return fmt.Errorf("vulnerable policies: expected transition %q, got %q", transitionName, result.Transition)
	}
	if !result.Sound {
		return fmt.Errorf("vulnerable policies: path is not sound")
	}
	if result.Problem != nil {
		return fmt.Errorf("vulnerable policies: cedar-woodpecker reported a problem: %v", result.Problem)
	}
	for _, required := range []string{
		`Action::"exfiltrate_data"`,
		`THVGroup::"engineering"`,
		`THVGroup::"support"`,
	} {
		if !strings.Contains(result.Policy, required) {
			return fmt.Errorf("vulnerable policies: implicit policy does not contain %s", required)
		}
	}
	return nil
}

// ValidateFixedResults ensures the fixed policies have no exfiltration path.
func ValidateFixedResults(fixed []Escalation) error {
	if len(fixed) != 0 {
		return fmt.Errorf("fixed policies: expected zero paths, got %d", len(fixed))
	}
	return nil
}

// ValidateVulnerableCubes checks the vulnerable Cedar source after
// cedar-woodpecker has parsed and typechecked it.
func ValidateVulnerableCubes(cubes []Cube) error {
	return validateCubeSet("vulnerable policies", cubes, vulnerableCubeExpectations())
}

// ValidateFixedCubes ensures remediation removes only the dangerous composed
// permission and preserves the intended engineering and support capabilities.
func ValidateFixedCubes(cubes []Cube) error {
	return validateCubeSet("fixed policies", cubes, fixedCubeExpectations())
}

type cubeExpectation struct {
	name      string
	condition string
}

func validateCubeSet(label string, cubes []Cube, expected []cubeExpectation) error {
	if len(cubes) != len(expected) {
		return fmt.Errorf("%s: expected exactly %d effective permission cubes, got %d", label, len(expected), len(cubes))
	}

	matched := make([]bool, len(expected))
	for _, cube := range cubes {
		if cube.Principal != "Client" || cube.Action != `Action::"call_tool"` || cube.Resource != "Tool" {
			return fmt.Errorf("%s: cube %q has unexpected request environment (%s, %s, %s)",
				label, cube.ID, cube.Principal, cube.Action, cube.Resource)
		}
		matchedIndex := -1
		for index, permission := range expected {
			if cube.Condition == permission.condition {
				matchedIndex = index
				break
			}
		}
		if matchedIndex == -1 {
			return fmt.Errorf("%s: unexpected effective cube %q: %s", label, cube.ID, cube.Condition)
		}
		if matched[matchedIndex] {
			return fmt.Errorf("%s: duplicate effective permission %s", label, expected[matchedIndex].name)
		}
		matched[matchedIndex] = true
	}
	for index, found := range matched {
		if !found {
			return fmt.Errorf("%s: missing effective permission %s", label, expected[index].name)
		}
	}
	return nil
}

func engineeringCubeExpectations() []cubeExpectation {
	return []cubeExpectation{{
		name:      "engineering list_resources",
		condition: `(principal in THVGroup::"engineering") && (resource == Tool::"list_resources")`,
	}}
}

func vulnerableCubeExpectations() []cubeExpectation {
	return append(engineeringCubeExpectations(), supportCubeExpectations()...)
}

func fixedCubeExpectations() []cubeExpectation {
	return append([]cubeExpectation{{
		name: "engineering list_resources excluding support",
		condition: `(principal in THVGroup::"engineering") && ((resource == Tool::"list_resources") && ` +
			`(!iferror(principal in THVGroup::"support", false)))`,
	}}, supportCubeExpectations()...)
}

func supportCubeExpectations() []cubeExpectation {
	return []cubeExpectation{
		{
			name:      "support add_issue_comment",
			condition: `(principal in THVGroup::"support") && (resource == Tool::"add_issue_comment")`,
		},
		{
			name:      "support issue_read",
			condition: `(principal in THVGroup::"support") && (resource == Tool::"issue_read")`,
		},
	}
}

// RenderImplicitPolicies creates the checked-in Cedar output for the paths.
func RenderImplicitPolicies(results []Escalation) ([]byte, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("cannot render an empty implicit policy set")
	}
	var output strings.Builder
	output.WriteString("// Code generated by go run ./cmd/analyze-exfiltration; DO NOT EDIT.\n\n")
	for index, result := range results {
		policy := strings.TrimSpace(result.Policy)
		if policy == "" {
			return nil, fmt.Errorf("result %d has an empty implicit policy", index)
		}
		if index > 0 {
			output.WriteString("\n\n")
		}
		output.WriteString(policy)
	}
	output.WriteByte('\n')
	return []byte(output.String()), nil
}

func requireVersion(ctx context.Context, path, expected string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("binary path is empty")
	}
	command := exec.CommandContext(ctx, path, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run %s --version: %w: %s", path, err, strings.TrimSpace(string(output)))
	}
	if !strings.Contains(string(output), expected) {
		return fmt.Errorf("%s is required, got %q", expected, strings.TrimSpace(string(output)))
	}
	return nil
}

func withEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, variable := range environment {
		if !strings.HasPrefix(variable, prefix) {
			result = append(result, variable)
		}
	}
	return append(result, prefix+value)
}
