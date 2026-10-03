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
	input := flag.String("input", "analysis/request-environments.json", "request classification file, relative to root")
	output := flag.String("output", "analysis/exfiltration-transitions.json", "generated transition file, relative to root")
	check := flag.Bool("check", false, "verify the generated transition file is current")
	flag.Parse()

	environments, err := demo.ReadRequestEnvironments(filepath.Join(*root, filepath.FromSlash(*input)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	transitions, err := demo.GenerateTransitions(environments)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data, err := demo.MarshalTransitions(transitions)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := demo.WriteGenerated(filepath.Join(*root, filepath.FromSlash(*output)), data, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d internal-data readers x %d public-internet writers = %d transitions\n",
		len(environments.InternalDataReaders), len(environments.PublicInternetWriters), len(transitions.Transitions))
}
