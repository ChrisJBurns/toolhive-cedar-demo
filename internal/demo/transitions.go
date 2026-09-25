// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// RequestEnvironments classifies exact Cedar requests by their data-flow role.
type RequestEnvironments struct {
	InternalDataReaders   []RequestEnvironment `json:"internal_data_readers"`
	PublicInternetWriters []RequestEnvironment `json:"public_internet_writers"`
	Target                TargetEnvironment    `json:"target"`
}

// RequestEnvironment is a typed Cedar request plus its exact resource entity.
type RequestEnvironment struct {
	Name         string `json:"name"`
	Principal    string `json:"principal"`
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	Resource     string `json:"resource"`
}

// TargetEnvironment is the analysis-only permission inferred by a transition.
type TargetEnvironment struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
}

// TransitionSet is cedar-woodpecker's transition file format.
type TransitionSet struct {
	Transitions []Transition `json:"transitions"`
}

// Transition composes source requests into an implicit target request.
type Transition struct {
	Name      string              `json:"name"`
	Principal []string            `json:"principal"`
	Sources   []TransitionRequest `json:"sources"`
	Target    TransitionRequest   `json:"target"`
	When      string              `json:"when"`
}

// TransitionRequest describes the Cedar types in a transition request.
type TransitionRequest struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

// ReadRequestEnvironments strictly decodes and validates the classifications.
func ReadRequestEnvironments(path string) (RequestEnvironments, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RequestEnvironments{}, fmt.Errorf("read request environments %s: %w", path, err)
	}
	var environments RequestEnvironments
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&environments); err != nil {
		return RequestEnvironments{}, fmt.Errorf("decode request environments %s: %w", path, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return RequestEnvironments{}, fmt.Errorf("decode request environments %s: %w", path, err)
	}
	if err := environments.Validate(); err != nil {
		return RequestEnvironments{}, fmt.Errorf("validate request environments %s: %w", path, err)
	}
	return environments, nil
}

// Validate checks that the complete Cartesian product is meaningful.
func (environments RequestEnvironments) Validate() error {
	if len(environments.InternalDataReaders) == 0 {
		return fmt.Errorf("internal_data_readers must not be empty")
	}
	if len(environments.PublicInternetWriters) == 0 {
		return fmt.Errorf("public_internet_writers must not be empty")
	}
	if strings.TrimSpace(environments.Target.Action) == "" {
		return fmt.Errorf("target.action must not be empty")
	}
	if strings.TrimSpace(environments.Target.ResourceType) == "" {
		return fmt.Errorf("target.resource_type must not be empty")
	}

	names := make(map[string]struct{})
	requests := append(append([]RequestEnvironment{}, environments.InternalDataReaders...), environments.PublicInternetWriters...)
	for _, request := range requests {
		if err := request.validate(); err != nil {
			return err
		}
		if _, exists := names[request.Name]; exists {
			return fmt.Errorf("request name %q is not unique", request.Name)
		}
		names[request.Name] = struct{}{}
	}
	for _, reader := range environments.InternalDataReaders {
		for _, writer := range environments.PublicInternetWriters {
			if reader.Principal != writer.Principal {
				return fmt.Errorf("requests %q and %q have different principal types", reader.Name, writer.Name)
			}
		}
	}
	return nil
}

func (request RequestEnvironment) validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "name", value: request.Name},
		{name: "principal", value: request.Principal},
		{name: "action", value: request.Action},
		{name: "resource_type", value: request.ResourceType},
		{name: "resource", value: request.Resource},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("request %q has an empty %s", request.Name, field.name)
		}
	}
	return nil
}

// GenerateTransitions creates the n*m data-flow transition functions.
func GenerateTransitions(environments RequestEnvironments) (TransitionSet, error) {
	if err := environments.Validate(); err != nil {
		return TransitionSet{}, err
	}
	transitions := make([]Transition, 0, len(environments.InternalDataReaders)*len(environments.PublicInternetWriters))
	for _, reader := range environments.InternalDataReaders {
		for _, writer := range environments.PublicInternetWriters {
			transitions = append(transitions, Transition{
				Name:      fmt.Sprintf("exfiltrate-via-%s-and-%s", reader.Name, writer.Name),
				Principal: []string{reader.Principal},
				Sources: []TransitionRequest{
					{Action: reader.Action, Resource: reader.ResourceType},
					{Action: writer.Action, Resource: writer.ResourceType},
				},
				Target: TransitionRequest{Action: environments.Target.Action, Resource: environments.Target.ResourceType},
				When:   fmt.Sprintf("context.resource1 == %s && context.resource2 == %s", reader.Resource, writer.Resource),
			})
		}
	}
	return TransitionSet{Transitions: transitions}, nil
}

// MarshalTransitions renders stable, human-readable JSON.
func MarshalTransitions(transitions TransitionSet) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(transitions); err != nil {
		return nil, fmt.Errorf("marshal transitions: %w", err)
	}
	return output.Bytes(), nil
}

// ReadTransitions strictly decodes a generated transition set.
func ReadTransitions(path string) (TransitionSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TransitionSet{}, fmt.Errorf("read transitions %s: %w", path, err)
	}
	var transitions TransitionSet
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transitions); err != nil {
		return TransitionSet{}, fmt.Errorf("decode transitions %s: %w", path, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return TransitionSet{}, fmt.Errorf("decode transitions %s: %w", path, err)
	}
	return transitions, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected data after JSON document")
		}
		return err
	}
	return nil
}
