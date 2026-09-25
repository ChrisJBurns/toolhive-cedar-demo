// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteGenerated writes a generated artifact atomically. In check mode it
// instead reports whether the checked-in artifact differs from data.
func WriteGenerated(path string, data []byte, check bool) error {
	if check {
		current, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read generated file %s: %w", path, err)
		}
		if !bytes.Equal(current, data) {
			return fmt.Errorf("generated file %s is out of date", path)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".generated-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // Best-effort cleanup after rename or failure.

	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck // Preserve the write error.
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close() //nolint:errcheck // Preserve the chmod error.
		return fmt.Errorf("set permissions on temporary file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace generated file %s: %w", path, err)
	}
	return nil
}

// ReadCedarFiles concatenates Cedar files in the supplied order.
func ReadCedarFiles(paths ...string) ([]byte, error) {
	var policies []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read Cedar policies %s: %w", path, err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil, fmt.Errorf("Cedar policy file %s is empty", path)
		}
		policies = append(policies, trimmed)
	}
	if len(policies) == 0 {
		return nil, fmt.Errorf("no Cedar policy files supplied")
	}
	return []byte(strings.Join(policies, "\n\n") + "\n"), nil
}
