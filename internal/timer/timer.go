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
	// PhaseBreak is the short rest phase between focus rounds.
	PhaseBreak
	// PhaseLongBreak is the longer rest phase that follows the final round. It is
	// a distinct phase rather than a longer break because it ends the split.
	PhaseLongBreak
)

// String renders the phase for display.
func (p Phase) String() string {
	switch p {
	case PhaseFocus:
		return "FOCUS"
	case PhaseBreak:
		return "BREAK"
	case PhaseLongBreak:
		return "LONG BREAK"
	case PhaseIdle:
		return "IDLE"
	default:
		return fmt.Sprintf("Phase(%d)", int(p))
	}
}

// Split is the plan a Timer runs: a number of focus rounds, each followed by a
// break, with the break after the last round made longer.
type Split struct {
	// Focus is how long each focus round lasts.
	Focus time.Duration
	// Break is the short rest between rounds.
	Break time.Duration
	// LongBreak is the rest after the final round.
	LongBreak time.Duration
	// Cycles is how many focus rounds the split runs. Zero or less still runs
	// one round and then ends, so a misconfigured split cannot run forever.
	Cycles int
}

// durationFor reports how long a phase of this split lasts.
func (s Split) durationFor(p Phase) time.Duration {
	switch p {
	case PhaseFocus:
		return s.Focus
	case PhaseBreak:
		return s.Break
	case PhaseLongBreak:
		return s.LongBreak
	default:
		return 0
	}
}

// Timer tracks a focus/break pair against the wall clock.
//
// It never reads the clock itself: every method that needs to know the current
// time takes now as an argument. That keeps the type deterministic and
// testable, and keeps rendering and playback out of it.
//
// Remaining time is always derived from an absolute deadline rather than
// decremented per tick, so a late or skipped update can never make the
// countdown drift.
type Timer struct {
	split    Split
	phase    Phase
	deadline time.Time

	// rounds counts the focus rounds already finished, which is what decides
	// whether the next break is the long one.
	rounds int

	// focused accumulates time spent in finished focus rounds, pauses excluded.
	focused time.Duration

	started bool

	paused          bool
	pausedRemaining time.Duration
}

// New starts a Timer in the focus phase, anchored to now.
func New(split Split, now time.Time) Timer {
	return Timer{
		split:    split,
		phase:    PhaseFocus,
		deadline: now.Add(split.Focus),
		started:  true,
	}
}

// Phase reports which phase is running, or PhaseIdle when none is.
func (t Timer) Phase() Phase { return t.phase }

// Started reports whether the Timer has been started and has not yet finished.
//
// The zero Timer is usable and reports neither running nor finished, so a caller
// that has not begun a split can tell the difference between "nothing is
// happening" and "the split is over".
func (t Timer) Started() bool { return t.started && t.phase != PhaseIdle }

// Rounds reports how many focus rounds have finished.
func (t Timer) Rounds() int { return t.rounds }

// Cycles reports how many focus rounds the split runs, never fewer than one.
//
// The floor is what the split actually runs, so a caller labelling the round in
// progress cannot end up claiming "round 1 of 0" for a split with no cycles set.
func (t Timer) Cycles() int { return t.cycles() }

// nextBreak reports which break follows the focus rounds finished so far: a
// short one between rounds, and a long one after the last, which is what ends
// the split.
func (t Timer) nextBreak() Phase {
	if t.rounds >= t.cycles() {
		return PhaseLongBreak
	}
	return PhaseBreak
}

// cycles reports how many focus rounds the split runs, never fewer than one.
func (t Timer) cycles() int { return max(t.split.Cycles, 1) }

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
func (t Timer) Finished() bool { return t.started && t.phase == PhaseIdle }

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

// Focused reports how much time has actually been spent working across every
// focus round so far, the one running included.
//
// It is not the same as how long the session has been open: time spent in a
// break, and time away from the keyboard while paused, is not work. A round
// skipped partway through only counts for the minutes it ran for.
func (t Timer) Focused(now time.Time) time.Duration {
	total := t.focused
	if t.phase == PhaseFocus {
		total += max(t.split.Focus-t.Remaining(now), 0)
	}
	return total
}

// Progress reports how far through the current phase the timer is, from 0 to 1.
//
// It stays frozen while paused, restarts from zero for each phase, and reads 1
// once every phase is done. A zero-length phase reads 1 rather than dividing by
// zero.
func (t Timer) Progress(now time.Time) float64 {
	total := t.split.durationFor(t.phase)
	if total <= 0 {
		return 1
	}

	fraction := 1 - float64(t.Remaining(now))/float64(total)
	return min(max(fraction, 0), 1)
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
//
// The next phase runs for its full length starting from now, which is what a
// deliberate skip should do. Automatic advancement when a phase runs out must
// use Advance instead, because that has to preserve the schedule.
func (t *Timer) Skip(now time.Time) {
	switch t.phase {
	case PhaseFocus:
		// Bank the round's work before moving on, while the remaining time still
		// describes the round that just ended.
		t.focused += max(t.split.Focus-t.Remaining(now), 0)
		t.rounds++
		t.phase = t.nextBreak()
		t.deadline = now.Add(t.split.durationFor(t.phase))
	case PhaseBreak:
		t.phase = PhaseFocus
		t.deadline = now.Add(t.split.Focus)
	case PhaseLongBreak:
		t.phase = PhaseIdle
		t.deadline = time.Time{}
	}
	t.paused = false
	t.pausedRemaining = 0
}

// Advance completes every phase that has run out and returns the phases it
// passed through, oldest first.
//
// Where Skip gives the next phase a fresh full duration measured from now,
// Advance keeps to the original schedule: each phase is anchored to the moment
// the previous one ended. So a phase that ran out while the process was
// suspended is followed immediately by the next one, and if that also ran out
// in the meantime it is reported in turn rather than quietly being handed
// another full duration nobody asked for.
//
// A paused timer is never advanced and keeps its pause: time spent away must
// not be able to end a phase the user deliberately stopped.
func (t *Timer) Advance(now time.Time) []Phase {
	var ended []Phase

	for t.Expired(now) {
		ended = append(ended, t.phase)
		// Anchoring to the old deadline rather than to now is the whole point.
		t.Skip(t.deadline)
	}

	return ended
}
