package timer

import (
	"fmt"
	"time"
)

// Phase identifies which part of a Split a Timer is currently running.
type Phase int

const (
	// PhaseIdle means no phase is running: either not yet started, or finished.
	PhaseIdle Phase = iota
	// PhaseFocus is the working phase.
	PhaseFocus
	// PhaseBreak is the rest phase that follows focus.
	PhaseBreak
)

// String renders the phase for display.
func (p Phase) String() string {
	switch p {
	case PhaseFocus:
		return "FOCUS"
	case PhaseBreak:
		return "BREAK"
	case PhaseIdle:
		return "IDLE"
	default:
		return fmt.Sprintf("Phase(%d)", int(p))
	}
}

// Split is the plan a Timer runs: how long to focus, then how long to rest.
type Split struct {
	Focus time.Duration
	Break time.Duration
}

// Timer tracks a focus/break pair against the wall clock.
//
// It never reads the clock itself: every method that needs to know the current
// time takes now as an argument. That keeps the type deterministic and
// testable, and keeps rendering and notification out of it.
//
// Remaining time is always derived from an absolute deadline rather than
// decremented per tick, so a late or skipped update can never make the
// countdown drift.
type Timer struct {
	split    Split
	phase    Phase
	deadline time.Time

	paused          bool
	pausedRemaining time.Duration
}

// New starts a Timer in the focus phase, anchored to now.
func New(split Split, now time.Time) Timer {
	return Timer{
		split:    split,
		phase:    PhaseFocus,
		deadline: now.Add(split.Focus),
	}
}

// Phase reports which phase is running, or PhaseIdle when none is.
func (t Timer) Phase() Phase { return t.phase }

// Expired reports whether the current phase has run out of time and is ready to
// be advanced with Skip.
//
// A paused timer never expires: pausing is meant to stop the countdown dead,
// however long the process stays suspended.
func (t Timer) Expired(now time.Time) bool {
	return t.phase != PhaseIdle && !t.paused && !t.deadline.After(now)
}

// Finished reports whether every phase has run to completion.
//
// This is deliberately not the same question as Expired: a phase that has run
// out still has to be advanced with Skip before the split is finished.
func (t Timer) Finished() bool { return t.phase == PhaseIdle }

// Paused reports whether the timer is holding still.
func (t Timer) Paused() bool { return t.paused }

// Remaining returns how much time is left in the current phase, never negative.
//
// While paused it reports the frozen value, so a paused timer keeps showing the
// same number however long the process stays suspended.
func (t Timer) Remaining(now time.Time) time.Duration {
	if t.paused {
		return t.pausedRemaining
	}
	if t.phase == PhaseIdle {
		return 0
	}
	if rem := t.deadline.Sub(now); rem > 0 {
		return rem
	}
	return 0
}

// Pause freezes the countdown at whatever is left.
//
// Pausing an already-paused timer does nothing, so a repeated keypress cannot
// quietly shorten the session.
func (t *Timer) Pause(now time.Time) {
	if t.paused {
		return
	}
	t.pausedRemaining = max(t.deadline.Sub(now), 0)
	t.paused = true
}

// Resume re-anchors the deadline to now, restoring the time that was frozen.
//
// This is what makes surviving a suspend work: after an hour away the timer
// still owes the full remainder rather than whatever the wall clock says is
// left of it.
func (t *Timer) Resume(now time.Time) {
	if !t.paused {
		return
	}
	t.deadline = now.Add(t.pausedRemaining)
	t.paused = false
	t.pausedRemaining = 0
}

// NextTick returns how long a renderer should wait before redrawing, so that
// redraws land on whole-second boundaries of the wall clock.
//
// Waiting a flat second after the last redraw would instead let each redraw's
// cost accumulate into the schedule, and the displayed countdown would fall
// progressively out of step with a real clock.
func NextTick(now time.Time) time.Duration {
	return now.Truncate(time.Second).Add(time.Second).Sub(now)
}

// Skip abandons the current phase and starts the next one.
//
// Skipping out of break finishes the split. Skipping also clears any pause, so
// the phase that follows always runs rather than inheriting a frozen clock.
func (t *Timer) Skip(now time.Time) {
	switch t.phase {
	case PhaseFocus:
		t.phase = PhaseBreak
		t.deadline = now.Add(t.split.Break)
	case PhaseBreak:
		t.phase = PhaseIdle
		t.deadline = time.Time{}
	}
	t.paused = false
	t.pausedRemaining = 0
}
