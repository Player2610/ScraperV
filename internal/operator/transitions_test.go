//go:build !integration

package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTransitionAllowed(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		// ─── valid transitions ───────────────────────────────────────────────
		{
			name: "pending_confirmation → confirmed",
			from: "pending_confirmation",
			to:   "confirmed",
			want: true,
		},
		{
			name: "pending_confirmation → cancelled",
			from: "pending_confirmation",
			to:   "cancelled",
			want: true,
		},
		{
			name: "confirmed → purchasing",
			from: "confirmed",
			to:   "purchasing",
			want: true,
		},
		{
			name: "confirmed → cancelled",
			from: "confirmed",
			to:   "cancelled",
			want: true,
		},
		{
			name: "purchasing → in_delivery",
			from: "purchasing",
			to:   "in_delivery",
			want: true,
		},
		{
			name: "in_delivery → delivered",
			from: "in_delivery",
			to:   "delivered",
			want: true,
		},

		// ─── terminal states — no outgoing transitions ───────────────────────
		{
			name: "delivered → confirmed (terminal)",
			from: "delivered",
			to:   "confirmed",
			want: false,
		},
		{
			name: "delivered → cancelled (terminal)",
			from: "delivered",
			to:   "cancelled",
			want: false,
		},
		{
			name: "delivered → purchasing (terminal)",
			from: "delivered",
			to:   "purchasing",
			want: false,
		},
		{
			name: "delivered → in_delivery (terminal)",
			from: "delivered",
			to:   "in_delivery",
			want: false,
		},
		{
			name: "cancelled → confirmed (terminal)",
			from: "cancelled",
			to:   "confirmed",
			want: false,
		},
		{
			name: "cancelled → purchasing (terminal)",
			from: "cancelled",
			to:   "purchasing",
			want: false,
		},
		{
			name: "cancelled → in_delivery (terminal)",
			from: "cancelled",
			to:   "in_delivery",
			want: false,
		},
		{
			name: "cancelled → delivered (terminal)",
			from: "cancelled",
			to:   "delivered",
			want: false,
		},

		// ─── invalid skip-ahead transitions ──────────────────────────────────
		{
			name: "pending_confirmation → in_delivery (skip)",
			from: "pending_confirmation",
			to:   "in_delivery",
			want: false,
		},
		{
			name: "pending_confirmation → delivered (skip)",
			from: "pending_confirmation",
			to:   "delivered",
			want: false,
		},
		{
			name: "confirmed → delivered (skip)",
			from: "confirmed",
			to:   "delivered",
			want: false,
		},

		// ─── invalid backward transitions ────────────────────────────────────
		{
			name: "purchasing → confirmed (backward)",
			from: "purchasing",
			to:   "confirmed",
			want: false,
		},
		{
			name: "in_delivery → purchasing (backward)",
			from: "in_delivery",
			to:   "purchasing",
			want: false,
		},
		{
			name: "in_delivery → confirmed (backward)",
			from: "in_delivery",
			to:   "confirmed",
			want: false,
		},
		{
			name: "in_delivery → pending_confirmation (backward)",
			from: "in_delivery",
			to:   "pending_confirmation",
			want: false,
		},

		// ─── unknown states ───────────────────────────────────────────────────
		{
			name: "unknown from state",
			from: "nonexistent",
			to:   "confirmed",
			want: false,
		},
		{
			name: "empty from state",
			from: "",
			to:   "confirmed",
			want: false,
		},
		{
			name: "empty to state",
			from: "pending_confirmation",
			to:   "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransitionAllowed(tt.from, tt.to)
			assert.Equal(t, tt.want, got, "isTransitionAllowed(%q, %q)", tt.from, tt.to)
		})
	}
}

// TestValidTransitions_MapCompleteness checks that all documented statuses have entries
// in the validTransitions map, including terminal states with empty slices.
func TestValidTransitions_MapCompleteness(t *testing.T) {
	expectedStatuses := []string{
		"pending_confirmation",
		"confirmed",
		"purchasing",
		"in_delivery",
		"delivered",
		"cancelled",
		"failed",
	}
	for _, s := range expectedStatuses {
		_, ok := validTransitions[s]
		assert.True(t, ok, "status %q should be present in validTransitions map", s)
	}
}

// TestIsTransitionAllowed_SelfTransitions verifies no status is allowed to transition
// to itself (idempotent self-transitions are not part of this state machine).
func TestIsTransitionAllowed_SelfTransitions(t *testing.T) {
	statuses := []string{
		"pending_confirmation",
		"confirmed",
		"purchasing",
		"in_delivery",
		"delivered",
		"cancelled",
		"failed",
	}
	for _, s := range statuses {
		t.Run("self:"+s, func(t *testing.T) {
			assert.False(t, isTransitionAllowed(s, s),
				"self-transition for %q should not be allowed", s)
		})
	}
}
