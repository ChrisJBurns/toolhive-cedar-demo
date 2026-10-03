// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChrisJBurns/toolhive-cedar-demo/internal/demo"
)

func main() {
	root := flag.String("root", ".", "repository root")
	schema := flag.String("schema", "policies/toolhive.cedarschema", "Cedar schema, relative to root")
	environmentsPath := flag.String("environments", "analysis/request-environments.json", "request classification file, relative to root")
	transitionsPath := flag.String("transitions", "analysis/exfiltration-transitions.json", "transition file, relative to root")
	output := flag.String("output", "policies/implicit/with-implicit-permissions.cedar", "generated implicit policies, relative to root")
	woodpecker := flag.String("cedar-woodpecker", ".state/bin/cedar-woodpecker", "cedar-woodpecker binary, relative to root")
	cvc5 := flag.String("cvc5", ".state/cvc5/bin/cvc5", "cvc5 binary, relative to root")
	check := flag.Bool("check", false, "verify the generated implicit policies are current")
	flag.Parse()

	transitionsFile := resolve(*root, *transitionsPath)
	transitions, err := demo.ReadTransitions(transitionsFile)
	if err != nil {
		fail(err)
	}
	if len(transitions.Transitions) != 1 {
		fail(fmt.Errorf("demo expects exactly one generated transition, got %d", len(transitions.Transitions)))
	}
	environments, err := demo.ReadRequestEnvironments(resolve(*root, *environmentsPath))
	if err != nil {
		fail(err)
	}

	vulnerable, err := demo.ReadCedarFiles(resolve(*root, "policies/20-combined-access.cedar"))
	if err != nil {
		fail(err)
	}
	toolHiveManifest, err := readFile(resolve(*root, "manifests/20-toolhive.yaml"))
	if err != nil {
		fail(err)
	}
	dexManifest, err := readFile(resolve(*root, "manifests/10-dex.yaml"))
	if err != nil {
		fail(err)
	}
	if err := demo.ValidateManifestBindings(environments, vulnerable, toolHiveManifest, dexManifest); err != nil {
		fail(err)
	}
	fixed, err := demo.ReadCedarFiles(resolve(*root, "policies/20-combined-access-fixed.cedar"))
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	analyzer := demo.Analyzer{Woodpecker: resolve(*root, *woodpecker), CVC5: resolve(*root, *cvc5)}
	if err := analyzer.ValidateTools(ctx); err != nil {
		fail(err)
	}
	vulnerableCubes, err := analyzer.Cubes(ctx, resolve(*root, *schema), vulnerable)
	if err != nil {
		fail(fmt.Errorf("inspect vulnerable policy permissions: %w", err))
	}
	fixedCubes, err := analyzer.Cubes(ctx, resolve(*root, *schema), fixed)
	if err != nil {
		fail(fmt.Errorf("inspect fixed policy permissions: %w", err))
	}
	if err := demo.ValidatePolicyCubes(vulnerableCubes, fixedCubes); err != nil {
		fail(err)
	}
	vulnerableResults, err := analyzer.Analyze(ctx, resolve(*root, *schema), transitionsFile, vulnerable)
	if err != nil {
		fail(fmt.Errorf("analyze vulnerable policies: %w", err))
	}
	fixedResults, err := analyzer.Analyze(ctx, resolve(*root, *schema), transitionsFile, fixed)
	if err != nil {
		fail(fmt.Errorf("analyze fixed policies: %w", err))
	}
	if err := demo.ValidateDemoResults(vulnerableResults, fixedResults, transitions.Transitions[0].Name); err != nil {
		fail(err)
	}
	implicit, err := demo.RenderImplicitPolicies(vulnerableResults)
	if err != nil {
		fail(err)
	}
	if err := demo.WriteGenerated(resolve(*root, *output), implicit, *check); err != nil {
		fail(err)
	}
	fmt.Print(renderReport(vulnerableResults[0]))
}

func renderReport(result demo.Escalation) string {
	return fmt.Sprintf(`Synthesized Cedar policy:

%s

Interpretation:
- engineering can call list_resources, which reads internal data.
- support can call add_issue_comment, which writes to the public internet.
- support-bot@example.com belongs to both groups, so it derives exfiltrate_data.
- The fixed policy has no exfiltration path.
`, strings.TrimSpace(result.Policy))
}

func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
