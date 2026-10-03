// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/ChrisJBurns/toolhive-cedar-demo/internal/demo"
)

func TestRenderReport(t *testing.T) {
	t.Parallel()
	policy := `permit(
  principal is Client,
  action == Action::"exfiltrate_data",
  resource is Exfiltration
) when {
  (principal in THVGroup::"engineering") && (principal in THVGroup::"support")
};`
	want := `Synthesized Cedar policy:

` + policy + `

Interpretation:
- engineering can call list_resources, which reads internal data.
- support can call add_issue_comment, which writes to the public internet.
- support-bot@example.com belongs to both groups, so it derives exfiltrate_data.
`

	got := renderReport(demo.Escalation{Policy: "\n" + policy + "\n"})
	if got != want {
		t.Fatalf("renderReport() = %q, want %q", got, want)
	}
	for _, oldOutput := range []string{"vulnerable policies:", "intended tool permissions retained", `"sound":true`} {
		if strings.Contains(got, oldOutput) {
			t.Errorf("renderReport() contains obsolete output %q", oldOutput)
		}
	}
}

func TestRenderFixedReport(t *testing.T) {
	t.Parallel()
	want := `Synthesized Cedar policy:

(none)

Interpretation:
- Cedar Woodpecker found 0 exfiltration paths.
- The fixed support boundary prevents the internal-data read from being combined with the public-internet write.
`
	if got := renderFixedReport(); got != want {
		t.Fatalf("renderFixedReport() = %q, want %q", got, want)
	}
}
