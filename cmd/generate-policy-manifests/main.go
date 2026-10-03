// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ChrisJBurns/toolhive-cedar-demo/internal/demo"
)

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.Bool("check", false, "verify generated manifests are current")
	flag.Parse()

	mappings := []struct {
		source string
		output string
	}{
		{"policies/00-deny-all.cedar", "policies/demo/00-deny-all.yaml"},
		{"policies/10-engineering.cedar", "policies/demo/10-engineering.yaml"},
		{"policies/20-combined-access.cedar", "policies/demo/20-combined-access.yaml"},
		{"policies/00-deny-all.cedar", "policies/demo-fixed/00-deny-all.yaml"},
		{"policies/10-engineering.cedar", "policies/demo-fixed/10-engineering.yaml"},
		{"policies/20-combined-access-fixed.cedar", "policies/demo-fixed/20-combined-access.yaml"},
	}

	for _, mapping := range mappings {
		source := filepath.Join(*root, filepath.FromSlash(mapping.source))
		output := filepath.Join(*root, filepath.FromSlash(mapping.output))
		if err := demo.GenerateToolHiveManifest(source, output, *check); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
