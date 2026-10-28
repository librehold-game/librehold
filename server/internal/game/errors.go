package game

import (
	"fmt"
	"strings"
)

// Blocker is one reason an order cannot run, as the client reads it: always "kind", plus the kind's fields.
type Blocker map[string]any

func (b Blocker) Kind() string {
	k, _ := b["kind"].(string)
	return k
}

// BlockedError is an order refused for the listed blockers.
type BlockedError struct {
	Op       string
	Blockers []Blocker
}

func (e *BlockedError) Error() string {
	kinds := make([]string, len(e.Blockers))
	for i, b := range e.Blockers {
		kinds[i] = b.Kind()
	}
	return fmt.Sprintf("%s: blocked (%s)", e.Op, strings.Join(kinds, ", "))
}

// HasKind reports whether any blocker has the given kind.
func (e *BlockedError) HasKind(kind string) bool {
	for _, b := range e.Blockers {
		if b.Kind() == kind {
			return true
		}
	}
	return false
}

func Blocked(op string, blockers ...Blocker) *BlockedError {
	return &BlockedError{Op: op, Blockers: blockers}
}

// RuleError is any other rule violation: a code the client translates, with the values its message needs.
type RuleError struct {
	Code   string
	Params map[string]any
}

func (e *RuleError) Error() string { return "rule: " + e.Code }

func Rule(code string, params map[string]any) *RuleError {
	return &RuleError{Code: code, Params: params}
}

// StoreError wraps an infrastructure failure inside a World method. It is raised with panic and recovered
// by the unit of work's caller (internal/app), never inside internal/game.
type StoreError struct{ Err error }

func (e *StoreError) Error() string { return "store: " + e.Err.Error() }

func (e *StoreError) Unwrap() error { return e.Err }

// NotOwner is the refusal of an order on a village (or command) the actor does not own.
func NotOwner(op string) *BlockedError {
	return Blocked(op, Blocker{"kind": "notOwner"})
}
