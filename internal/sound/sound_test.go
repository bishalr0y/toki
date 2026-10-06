package sound

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// stub swaps the two seams the package uses to reach the outside world, so no
// test ever inspects the developer's PATH or makes a noise.
type stub struct {
	available map[string]bool
	ran       []string
	ranArgs   [][]string
	fail      bool
}

func (s *stub) install(t *testing.T) {
	t.Helper()

	restoreLook, restoreRun := lookPath, run
	t.Cleanup(func() { lookPath, run = restoreLook, restoreRun })

	lookPath = func(name string) (string, error) {
		if s.available[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	run = func(name string, args ...string) error {
		s.ran = append(s.ran, name)
		s.ranArgs = append(s.ranArgs, args)
		if s.fail {
			return errors.New("player failed")
		}
		return nil
	}
}

func touch(t *testing.T, dir, name string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte("sound"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A completion sound can be any of a few names; the one listed first wins, so a
// user can drop in several and know which will play without guessing.
func TestFindPrefersTheFirstNameInTheList(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "toki.mp3")
	touch(t, dir, "complete.wav")

	got, ok := Find(dir)
	if !ok {
		t.Fatal("Find found no sound, want complete.wav")
	}
	if want := filepath.Join(dir, "complete.wav"); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

func TestFindReturnsTheOnlySoundThereIs(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "toki.ogg")

	got, ok := Find(dir)
	if !ok {
		t.Fatal("Find found no sound, want toki.ogg")
	}
	if want := filepath.Join(dir, "toki.ogg"); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

// The absence of a sound is the normal case: nobody owes toki a file, and
// nothing should be reported when there is none.
func TestFindReportsNoSoundInAnEmptyDirectory(t *testing.T) {
	if got, ok := Find(t.TempDir()); ok {
		t.Errorf("Find = %q, true; want no sound at all", got)
	}
}

// A directory named complete.wav is not a sound, and trying to play it would
// hand a directory to the player and fail.
func TestFindIgnoresADirectoryNamedLikeASound(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "complete.wav"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got, ok := Find(dir); ok {
		t.Errorf("Find = %q, true; want a directory to be ignored", got)
	}
}

// Play must run whichever player is actually installed, and pass it the sound.
func TestPlayRunsTheFirstAvailablePlayerWithTheSound(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "complete.wav")

	s := &stub{available: map[string]bool{"aplay": true, "mpv": true}}
	s.install(t)

	Play(dir)

	if len(s.ran) != 1 {
		t.Fatalf("ran %v, want exactly one player", s.ran)
	}
	if s.ran[0] != "aplay" {
		t.Errorf("ran %q, want aplay, the first available player", s.ran[0])
	}
	if len(s.ranArgs[0]) == 0 || s.ranArgs[0][len(s.ranArgs[0])-1] != filepath.Join(dir, "complete.wav") {
		t.Errorf("args = %v, want the sound path last", s.ranArgs[0])
	}
}

// With no sound file there is nothing to play, and no player should be run.
func TestPlayStaysSilentWithoutASound(t *testing.T) {
	s := &stub{available: map[string]bool{"afplay": true}}
	s.install(t)

	Play(t.TempDir())

	if len(s.ran) != 0 {
		t.Errorf("ran %v, want nothing", s.ran)
	}
}

// A sound but no player on a minimal system is not an error worth surfacing.
func TestPlayStaysSilentWithoutAPlayer(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "complete.wav")

	s := &stub{available: map[string]bool{}}
	s.install(t)

	Play(dir)

	if len(s.ran) != 0 {
		t.Errorf("ran %v, want nothing", s.ran)
	}
}

// A player that fails must not panic or retry forever; best-effort means the
// failure is swallowed and the timer carries on.
func TestPlaySwallowsAFailingPlayer(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "complete.wav")

	s := &stub{available: map[string]bool{"afplay": true}, fail: true}
	s.install(t)

	Play(dir) // must not panic

	if len(s.ran) != 1 {
		t.Errorf("ran %d players, want the failing one tried once", len(s.ran))
	}
}
