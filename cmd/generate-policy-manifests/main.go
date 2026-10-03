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
		{"policies/20-combined-access.cedar", "policies/demo/combined-access.yaml"},
		{"policies/20-combined-access-fixed.cedar", "policies/demo-fixed/combined-access.yaml"},
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
