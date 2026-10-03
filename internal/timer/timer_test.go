package timer

import (
	"math"
	"slices"
	"testing"
	"time"
)

// start is a fixed instant so every assertion below is an absolute clock
// reading rather than a value recomputed the way the code computes it.
var start = time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)

func standardSplit() Split {
	return Split{Focus: 25 * time.Minute, Break: 5 * time.Minute}
}

func TestNewStartsInFocusWithTheFullDuration(t *testing.T) {
	tr := New(standardSplit(), start)

	if got := tr.Phase(); got != PhaseFocus {
		t.Errorf("Phase() = %v, want %v", got, PhaseFocus)
	}
	if got := tr.Remaining(start); got != 25*time.Minute {
		t.Errorf("Remaining(start) = %v, want 25m", got)
	}
	if tr.Finished() {
		t.Error("Finished() = true immediately after New, want false")
	}
}

func TestRemainingTracksTheWallClock(t *testing.T) {
	tr := New(standardSplit(), start)

	steps := []struct {
		after time.Duration
		want  time.Duration
	}{
		{0, 25 * time.Minute},
		{time.Second, 24*time.Minute + 59*time.Second},
		{time.Minute, 24 * time.Minute},
		{10 * time.Minute, 15 * time.Minute},
		{24*time.Minute + 59*time.Second, time.Second},
		{25 * time.Minute, 0},
		{40 * time.Minute, 0}, // past the deadline: clamps, never negative
	}

	for _, s := range steps {
		if got := tr.Remaining(start.Add(s.after)); got != s.want {
			t.Errorf("after %v: Remaining = %v, want %v", s.after, got, s.want)
		}
	}
}

// Remaining must be a pure function of the instant passed in. The original
// implementation counted ticks, so its answer drifted with update frequency;
// sampling the same instant repeatedly must never move it.
func TestRemainingIsPureAndRepeatable(t *testing.T) {
	tr := New(standardSplit(), start)
	at := start.Add(7*time.Minute + 23*time.Second)
	first := tr.Remaining(at)

	for i := range 1000 {
		if got := tr.Remaining(at); got != first {
			t.Fatalf("call %d: Remaining = %v, want stable %v", i, got, first)
		}
	}
}

func TestPauseFreezesTheCountdown(t *testing.T) {
	tr := New(standardSplit(), start)

	pausedAt := start.Add(10 * time.Minute)
	tr.Pause(pausedAt)

	if !tr.Paused() {
		t.Error("Paused() = false after Pause, want true")
	}
	if got := tr.Remaining(pausedAt); got != 15*time.Minute {
		t.Errorf("Remaining at pause = %v, want 15m", got)
	}

	// An hour of wall-clock time passes. A paused timer must not care.
	afterSuspension := pausedAt.Add(time.Hour)
	if got := tr.Remaining(afterSuspension); got != 15*time.Minute {
		t.Errorf("Remaining an hour into a pause = %v, want a frozen 15m", got)
	}
}

func TestResumeRestoresTheFullRemainingTime(t *testing.T) {
	tr := New(standardSplit(), start)

	pausedAt := start.Add(10 * time.Minute)
	tr.Pause(pausedAt)

	// Suspended for an hour with the lid closed, then resumed.
	resumedAt := pausedAt.Add(time.Hour)
	tr.Resume(resumedAt)

	if tr.Paused() {
		t.Error("Paused() = true after Resume, want false")
	}
	if got := tr.Remaining(resumedAt); got != 15*time.Minute {
		t.Errorf("Remaining at resume = %v, want the full 15m", got)
	}
	if got := tr.Remaining(resumedAt.Add(14 * time.Minute)); got != time.Minute {
		t.Errorf("Remaining 14m after resume = %v, want 1m", got)
	}
}

func TestPauseAndResumeAreIdempotent(t *testing.T) {
	tr := New(standardSplit(), start)

	pausedAt := start.Add(10 * time.Minute)
	tr.Pause(pausedAt)
	tr.Pause(pausedAt.Add(time.Minute))
	if got := tr.Remaining(pausedAt.Add(time.Minute)); got != 15*time.Minute {
		t.Errorf("second Pause moved the clock: Remaining = %v, want 15m", got)
	}

	resumedAt := pausedAt.Add(time.Hour)
	tr.Resume(resumedAt)
	tr.Resume(resumedAt.Add(time.Minute))
	if got := tr.Remaining(resumedAt.Add(time.Minute)); got != 14*time.Minute {
		t.Errorf("second Resume restarted the deadline: Remaining = %v, want 14m", got)
	}
}

// "Is this phase over" and "is the whole split done" are separate questions, so
// they get separate methods: a runner asks Expired to know when to advance, and
// Finished to know when to stop.
func TestExpiredTracksTheCurrentPhase(t *testing.T) {
	tr := New(standardSplit(), start)

	if tr.Expired(start) {
		t.Error("Expired at the very start, want false")
	}
	if tr.Expired(start.Add(24*time.Minute + 59*time.Second)) {
		t.Error("Expired one second early, want false")
	}
	if !tr.Expired(start.Add(25 * time.Minute)) {
		t.Error("Expired = false at the deadline, want true")
	}
}

func TestAPausedTimerNeverExpires(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Pause(start.Add(10 * time.Minute))

	if tr.Expired(start.Add(25*time.Minute + 48*time.Hour)) {
		t.Error("a paused timer expired; pausing must stop the countdown entirely")
	}
}

func TestSkipMovesFromFocusToBreak(t *testing.T) {
	tr := New(standardSplit(), start)

	skippedAt := start.Add(3 * time.Minute)
	tr.Skip(skippedAt)

	if got := tr.Phase(); got != PhaseBreak {
		t.Errorf("Phase() after Skip = %v, want %v", got, PhaseBreak)
	}
	if got := tr.Remaining(skippedAt); got != 5*time.Minute {
		t.Errorf("Remaining after Skip = %v, want the full 5m break", got)
	}
	if tr.Finished() {
		t.Error("Finished() = true after one Skip, want false")
	}
}

func TestBreakRunningOutDoesNotFinishTheSplitOnItsOwn(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Skip(start)

	if got := tr.Remaining(start.Add(4*time.Minute + 59*time.Second)); got != time.Second {
		t.Errorf("Remaining late in the break = %v, want 1s", got)
	}
	if !tr.Expired(start.Add(5 * time.Minute)) {
		t.Error("break should be expired once its 5m are up")
	}
	if tr.Finished() {
		t.Error("Finished() = true while still in PhaseBreak, want false until advanced")
	}
}

func TestSkipFromBreakFinishesTheSplit(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Skip(start)
	tr.Skip(start.Add(5 * time.Minute))

	if got := tr.Phase(); got != PhaseIdle {
		t.Errorf("Phase() = %v, want %v", got, PhaseIdle)
	}
	if !tr.Finished() {
		t.Error("Finished() = false after both phases, want true")
	}
	if got := tr.Remaining(start.Add(5 * time.Minute)); got != 0 {
		t.Errorf("Remaining when finished = %v, want 0", got)
	}
}

func TestSkipClearsAPause(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Pause(start.Add(10 * time.Minute))
	tr.Skip(start.Add(10 * time.Minute))

	if tr.Paused() {
		t.Error("Paused() = true after Skip, want false; the next phase must run")
	}
	if got := tr.Remaining(start.Add(10 * time.Minute)); got != 5*time.Minute {
		t.Errorf("Remaining after skipping from a pause = %v, want 5m", got)
	}
}

func TestAdvanceDoesNothingWhileThePhaseIsStillRunning(t *testing.T) {
	tr := New(standardSplit(), start)

	if ended := tr.Advance(start.Add(10 * time.Minute)); len(ended) != 0 {
		t.Errorf("Advance mid-phase returned %v, want nothing", ended)
	}
	if tr.Phase() != PhaseFocus {
		t.Errorf("Phase() = %v, want %v; an unfinished phase must not advance", tr.Phase(), PhaseFocus)
	}
}

// The break runs for five minutes from the moment focus ended, not from
// whenever the update happened to arrive.
func TestAdvanceKeepsTheScheduleRatherThanRestartingFromNow(t *testing.T) {
	tr := New(standardSplit(), start)

	// Two seconds late to notice that focus is over.
	late := start.Add(25*time.Minute + 2*time.Second)
	ended := tr.Advance(late)

	if len(ended) != 1 || ended[0] != PhaseFocus {
		t.Fatalf("Advance returned %v, want [%v]", ended, PhaseFocus)
	}
	if got, want := tr.Remaining(late), 5*time.Minute-2*time.Second; got != want {
		t.Errorf("Remaining = %v, want %v; the break should end at start+30m, not now+5m", got, want)
	}
}

// A phase that ran out while the process was suspended is followed straight
// away by the next, and one that also ran out is reported rather than being
// granted a fresh full duration.
func TestAdvanceCascadesEveryPhaseMissedDuringASuspend(t *testing.T) {
	tr := New(standardSplit(), start)

	// Suspended for half an hour: both phases ran out.
	ended := tr.Advance(start.Add(30 * time.Minute))

	want := []Phase{PhaseFocus, PhaseBreak}
	if !slices.Equal(ended, want) {
		t.Fatalf("Advance returned %v, want %v", ended, want)
	}
	if !tr.Finished() {
		t.Error("Finished() = false after both phases ran out, want true")
	}
}

func TestAdvanceDoesNotRestartTheBreakAfterALongSuspend(t *testing.T) {
	tr := New(standardSplit(), start)

	// Ten minutes into a twenty five minute focus: still running.
	if ended := tr.Advance(start.Add(10 * time.Minute)); len(ended) != 0 {
		t.Fatalf("Advance returned %v, want nothing", ended)
	}
	// Another sixteen minutes pass, ending focus one minute ago. The break
	// therefore owes four of its five minutes, not five.
	resumed := start.Add(26 * time.Minute)
	tr.Advance(resumed)

	if got, want := tr.Remaining(resumed), 4*time.Minute; got != want {
		t.Errorf("Remaining after resuming = %v, want %v", got, want)
	}
}

// Pausing is meant to survive any length of absence, so a paused timer must
// never advance itself, however late the update arrives.
func TestAdvanceLeavesAPausedTimerAlone(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Pause(start.Add(10 * time.Minute))

	if ended := tr.Advance(start.Add(4 * time.Hour)); len(ended) != 0 {
		t.Errorf("Advance returned %v while paused, want nothing", ended)
	}
	if !tr.Paused() {
		t.Error("Paused() = false after Advance, want true; a tick must not unpause")
	}
	if got, want := tr.Remaining(start.Add(4*time.Hour)), 15*time.Minute; got != want {
		t.Errorf("Remaining = %v, want a frozen %v", got, want)
	}
}

// Ticking one second after the last update makes each update inherit the cost of
// rendering, so the displayed second creeps out of step with the wall clock.
// Snapping to whole seconds is the fix.
// Pausing an already-overrun timer must freeze at zero rather than banking a
// negative remainder that Resume would then have to undo.
func TestPauseAfterTheDeadlineFreezesAtZero(t *testing.T) {
	tr := New(standardSplit(), start)

	overran := start.Add(30 * time.Minute) // five minutes past the focus deadline
	tr.Pause(overran)

	if got := tr.Remaining(overran); got != 0 {
		t.Errorf("Remaining when pausing late = %v, want 0", got)
	}

	tr.Resume(overran.Add(time.Minute))
	if got := tr.Remaining(overran.Add(time.Minute)); got != 0 {
		t.Errorf("Remaining after resuming a late pause = %v, want 0", got)
	}
}

func TestPhaseStringNamesEachPhase(t *testing.T) {
	steps := []struct {
		phase Phase
		want  string
	}{
		{PhaseIdle, "IDLE"},
		{PhaseFocus, "FOCUS"},
		{PhaseBreak, "BREAK"},
	}

	for _, s := range steps {
		if got := s.phase.String(); got != s.want {
			t.Errorf("Phase(%d).String() = %q, want %q", int(s.phase), got, s.want)
		}
	}

	// An unrecognised value must still render as something identifiable rather
	// than an empty label.
	if got := Phase(99).String(); got == "" {
		t.Error("Phase(99).String() = \"\", want a non-empty label")
	}
}

func TestProgressTracksThePhaseFromZeroToOne(t *testing.T) {
	tr := New(standardSplit(), start)

	steps := []struct {
		after time.Duration
		want  float64
		tol   float64
	}{
		{0, 0, 1e-9},
		// A second into a 25 minute phase is 1/1500 of the way through, not
		// literally nothing.
		{time.Second, 0, 0.001},
		{12*time.Minute + 30*time.Second, 0.5, 1e-9},
		{25 * time.Minute, 1, 1e-9},
		{40 * time.Minute, 1, 1e-9}, // overrun clamps rather than exceeding 1
	}

	for _, s := range steps {
		if got := tr.Progress(start.Add(s.after)); math.Abs(got-s.want) > s.tol {
			t.Errorf("after %v: Progress = %v, want %v (+/- %v)", s.after, got, s.want, s.tol)
		}
	}
}

func TestProgressRestartsForEachPhase(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Skip(start) // into the 5 minute break

	if got := tr.Progress(start); !closeEnough(got, 0) {
		t.Errorf("Progress at the start of break = %v, want 0", got)
	}
	if got := tr.Progress(start.Add(2*time.Minute + 30*time.Second)); !closeEnough(got, 0.5) {
		t.Errorf("Progress halfway through a 5m break = %v, want 0.5", got)
	}
}

func TestProgressFreezesWhilePaused(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Pause(start.Add(12*time.Minute + 30*time.Second))

	if before := tr.Progress(start.Add(12*time.Minute + 30*time.Second)); !closeEnough(before, 0.5) {
		t.Fatalf("Progress at pause = %v, want 0.5", before)
	}

	// Time passes during the pause; the bar must not move.
	if got := tr.Progress(start.Add(40 * time.Minute)); !closeEnough(got, 0.5) {
		t.Errorf("Progress during a pause = %v, want a frozen 0.5", got)
	}
}

// With no phase running there is nothing left to do, so the bar reads full.
func TestProgressIsFullWhenThereIsNoPhase(t *testing.T) {
	tr := New(standardSplit(), start)
	tr.Skip(start)
	tr.Skip(start)

	if got := tr.Progress(start); !closeEnough(got, 1) {
		t.Errorf("Progress when finished = %v, want 1", got)
	}
}

// A zero-length phase must not divide by zero.
func TestProgressHandlesAZeroLengthPhase(t *testing.T) {
	tr := New(Split{Focus: 0, Break: 0}, start)

	if got := tr.Progress(start); !closeEnough(got, 1) {
		t.Errorf("Progress with no duration = %v, want 1", got)
	}
}

func closeEnough(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

func TestNextTickLandsOnTheNextWholeSecond(t *testing.T) {
	steps := []struct {
		now  time.Time
		want time.Duration
	}{
		{start, time.Second},
		{start.Add(340 * time.Millisecond), 660 * time.Millisecond},
		{start.Add(500 * time.Millisecond), 500 * time.Millisecond},
		{start.Add(999 * time.Millisecond), time.Millisecond},
	}

	for _, s := range steps {
		if got := NextTick(s.now); got != s.want {
			t.Errorf("NextTick(%v) = %v, want %v", s.now, got, s.want)
		}
	}
}

// The absolute invariant, swept across a whole second of possible inputs: the
// instant a tick fires is always exactly on a second boundary, and always
// strictly after now.
func TestNextTickAlwaysLandsOnAWholeSecondBoundary(t *testing.T) {
	for ns := int64(0); ns < int64(time.Second); ns += int64(time.Millisecond) {
		now := start.Add(time.Duration(ns))
		wait := NextTick(now)
		fires := now.Add(wait)

		if fires.Nanosecond() != 0 {
			t.Fatalf("NextTick from %v fires at %v, which is not a whole second", now, fires)
		}
		if !fires.After(now) {
			t.Fatalf("NextTick from %v returned %v, which does not advance time", now, wait)
		}
	}
}
