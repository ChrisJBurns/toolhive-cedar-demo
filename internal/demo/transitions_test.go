// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"strings"
	"testing"
)

func TestGenerateTransitionsBuildsCartesianProduct(t *testing.T) {
	t.Parallel()
	environments := RequestEnvironments{
		InternalDataReaders: []RequestEnvironment{
			{Name: "read-a", Principal: "Client", Action: `Action::"call_tool"`, ResourceType: "Tool", Resource: `Tool::"read_a"`},
			{Name: "read-b", Principal: "Client", Action: `Action::"call_tool"`, ResourceType: "Tool", Resource: `Tool::"read_b"`},
		},
		PublicInternetWriters: []RequestEnvironment{
			{Name: "write", Principal: "Client", Action: `Action::"call_tool"`, ResourceType: "Tool", Resource: `Tool::"write"`},
		},
		Target: TargetEnvironment{Action: `Action::"exfiltrate_data"`, ResourceType: "Exfiltration"},
	}

	set, err := GenerateTransitions(environments)
	if err != nil {
		t.Fatalf("GenerateTransitions() error = %v", err)
	}
	if len(set.Transitions) != 2 {
		t.Fatalf("len(Transitions) = %d, want 2", len(set.Transitions))
	}
	if got, want := set.Transitions[0].Name, "exfiltrate-via-read-a-and-write"; got != want {
		t.Errorf("first transition name = %q, want %q", got, want)
	}
	if got := set.Transitions[0].When; strings.Contains(got, "public_internet") {
		t.Errorf("transition when clause unnecessarily fixes a target entity: %q", got)
	}
	data, err := MarshalTransitions(set)
	if err != nil {
		t.Fatalf("MarshalTransitions() error = %v", err)
	}
	if strings.Contains(string(data), `\u0026`) || !strings.Contains(string(data), " && ") {
		t.Errorf("MarshalTransitions() escaped the readable Cedar expression:\n%s", data)
	}
}

func TestRequestEnvironmentsRejectsDuplicateNames(t *testing.T) {
	t.Parallel()
	request := RequestEnvironment{Name: "same", Principal: "Client", Action: "action", ResourceType: "Tool", Resource: "resource"}
	environments := RequestEnvironments{
		InternalDataReaders:   []RequestEnvironment{request},
		PublicInternetWriters: []RequestEnvironment{request},
		Target:                TargetEnvironment{Action: "target", ResourceType: "Exfiltration"},
	}

	err := environments.Validate()
	if err == nil || !strings.Contains(err.Error(), "not unique") {
		t.Fatalf("Validate() error = %v, want duplicate-name error", err)
	}
}
