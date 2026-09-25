// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"strings"
	"testing"
)

func TestValidateDemoResults(t *testing.T) {
	t.Parallel()
	vulnerable := []Escalation{{
		Transition: "exfiltrate-via-list_resources-and-add_issue_comment",
		Sound:      true,
		Policy: `permit(principal, Action::"exfiltrate_data", resource) when {
principal in THVGroup::"engineering" && principal in THVGroup::"support"
};`,
	}}

	if err := ValidateDemoResults(vulnerable, nil, vulnerable[0].Transition); err != nil {
		t.Fatalf("ValidateDemoResults() error = %v", err)
	}
	if err := ValidateDemoResults(vulnerable, vulnerable, vulnerable[0].Transition); err == nil || !strings.Contains(err.Error(), "expected zero") {
		t.Fatalf("ValidateDemoResults() fixed error = %v, want non-zero fixed-path error", err)
	}
}

func TestRenderImplicitPolicies(t *testing.T) {
	t.Parallel()
	output, err := RenderImplicitPolicies([]Escalation{{Policy: "permit(principal, action, resource);"}})
	if err != nil {
		t.Fatalf("RenderImplicitPolicies() error = %v", err)
	}
	if !strings.HasPrefix(string(output), "// Code generated") || !strings.HasSuffix(string(output), ";\n") {
		t.Fatalf("RenderImplicitPolicies() = %q", output)
	}
}

func TestValidateFixedCubes(t *testing.T) {
	t.Parallel()
	cubes := []Cube{
		fixedCube("engineering-read", `(principal in THVGroup::"engineering") && ((resource == Tool::"list_resources") && (!iferror(principal in THVGroup::"support", false)))`),
		fixedCube("support-comment", `(principal in THVGroup::"support") && (resource == Tool::"add_issue_comment")`),
		fixedCube("support-read", `(principal in THVGroup::"support") && (resource == Tool::"issue_read")`),
	}
	if err := ValidateFixedCubes(cubes); err != nil {
		t.Fatalf("ValidateFixedCubes() error = %v", err)
	}
}

func TestValidateFixedCubesRejectsUnconstrainedEngineeringRead(t *testing.T) {
	t.Parallel()
	cubes := []Cube{
		fixedCube("engineering-read", `(principal in THVGroup::"engineering") && (resource == Tool::"list_resources")`),
		fixedCube("support-comment", `(principal in THVGroup::"support") && (resource == Tool::"add_issue_comment")`),
		fixedCube("support-read", `(principal in THVGroup::"support") && (resource == Tool::"issue_read")`),
	}
	if err := ValidateFixedCubes(cubes); err == nil {
		t.Fatal("ValidateFixedCubes() error = nil, want missing support-role exclusion rejection")
	}
}

func TestValidateStagedPolicyCubes(t *testing.T) {
	t.Parallel()
	engineering := []Cube{fixedCube("engineering-read", engineeringCubeExpectations()[0].condition)}
	vulnerable := append(append([]Cube{}, engineering...), []Cube{
		fixedCube("support-comment", supportCubeExpectations()[0].condition),
		fixedCube("support-read", supportCubeExpectations()[1].condition),
	}...)
	fixed := []Cube{
		fixedCube("engineering-read-constrained", fixedCubeExpectations()[0].condition),
		fixedCube("support-comment", supportCubeExpectations()[0].condition),
		fixedCube("support-read", supportCubeExpectations()[1].condition),
	}
	if err := ValidateStagedPolicyCubes(nil, engineering, vulnerable, fixed); err != nil {
		t.Fatalf("ValidateStagedPolicyCubes() error = %v", err)
	}
}

func TestValidateStagedPolicyCubesRejectsDenyAllPermission(t *testing.T) {
	t.Parallel()
	unexpected := []Cube{fixedCube("unexpected", engineeringCubeExpectations()[0].condition)}
	if err := ValidateStagedPolicyCubes(unexpected, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "deny-all") {
		t.Fatalf("ValidateStagedPolicyCubes() error = %v, want deny-all cube error", err)
	}
}

func fixedCube(id, condition string) Cube {
	return Cube{
		Principal: "Client",
		Action:    `Action::"call_tool"`,
		Resource:  "Tool",
		ID:        id,
		Condition: condition,
		Outcomes:  []string{"True", "False"},
	}
}
