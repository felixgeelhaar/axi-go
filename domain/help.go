package domain

import (
	"fmt"
	"strings"
)

// Help returns a human-readable summary of the action suitable for display
// when an agent or operator asks "what is this?". Aligned with axi.md
// principle #10 (consistent way to get help).
//
// The format includes name, description, effect/idempotency, input/output
// contracts, and required capabilities.
func (a *ActionDefinition) Help() string {
	var sb strings.Builder
	if a.description != "" {
		fmt.Fprintf(&sb, "%s — %s\n", a.name, a.description)
	} else {
		fmt.Fprintf(&sb, "%s\n", a.name)
	}
	fmt.Fprintf(&sb, "Effect: %s  Idempotent: %t\n", a.effectProfile.Level, a.idempotencyProfile.IsIdempotent)

	sb.WriteString("\nInput:\n")
	writeContract(&sb, a.inputContract)

	sb.WriteString("\nOutput:\n")
	writeContract(&sb, a.outputContract)

	if len(a.requirements) > 0 {
		sb.WriteString("\nRequires capabilities:\n")
		for _, r := range a.requirements {
			fmt.Fprintf(&sb, "  - %s\n", r.Capability)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// Help returns a human-readable summary of the capability. Aligned with
// axi.md principle #10.
func (c *CapabilityDefinition) Help() string {
	var sb strings.Builder
	if c.description != "" {
		fmt.Fprintf(&sb, "%s — %s\n", c.name, c.description)
	} else {
		fmt.Fprintf(&sb, "%s\n", c.name)
	}
	sb.WriteString("\nInput:\n")
	writeContract(&sb, c.inputContract)
	sb.WriteString("\nOutput:\n")
	writeContract(&sb, c.outputContract)
	return strings.TrimRight(sb.String(), "\n")
}

func writeContract(sb *strings.Builder, c Contract) {
	if c.IsEmpty() {
		sb.WriteString("  (no fields)\n")
		return
	}
	for _, f := range c.Fields {
		kind := f.Type
		if kind == "" {
			kind = "any"
		}
		req := "optional"
		if f.Required {
			req = "required"
		}
		if f.Description != "" {
			fmt.Fprintf(sb, "  %s  (%s, %s)  %s\n", f.Name, kind, req, f.Description)
		} else {
			fmt.Fprintf(sb, "  %s  (%s, %s)\n", f.Name, kind, req)
		}
		if f.Example != nil {
			fmt.Fprintf(sb, "    example: %v\n", f.Example)
		}
	}
}
