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

// The default sound is embedded in the binary so a fresh install is not silent.
func TestDefaultIsAValidWAV(t *testing.T) {
	data := Default()
	if len(data) < 44 {
		t.Fatalf("default sound is %d bytes, too small to be a WAV", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Errorf("default sound is not a RIFF/WAVE file: %q", data[:12])
	}
}

// The completion sound has exactly one name, whatever its format, so there is
// nothing to choose between and nothing to remember.
func TestFindReturnsTheCompletionSound(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, DefaultFile)

	got, ok := Find(dir)
	if !ok {
		t.Fatalf("Find found no sound, want %s", DefaultFile)
	}
	if want := filepath.Join(dir, DefaultFile); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

// A WAV is not the only format that counts: an MP3 the user dropped in is found
// when there is no WAV, so the one name works in either format.
func TestFindFindsAnMP3WhenThereIsNoWAV(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "sound.mp3")

	got, ok := Find(dir)
	if !ok {
		t.Fatal("Find found no sound, want sound.mp3")
	}
	if want := filepath.Join(dir, "sound.mp3"); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

// When both formats are present the shipped default wins, so the choice is
// deterministic rather than whichever file the filesystem happens to list first.
func TestFindPrefersTheWAVWhenBothFormatsArePresent(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "sound.mp3")
	touch(t, dir, DefaultFile)

	got, ok := Find(dir)
	if !ok {
		t.Fatal("Find found no sound")
	}
	if want := filepath.Join(dir, DefaultFile); got != want {
		t.Errorf("Find = %q, want the WAV %q", got, want)
	}
}

// Other audio files sitting in the config directory are not the completion
// sound; only the one name counts.
func TestFindIgnoresEveryOtherName(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "toki.mp3")
	touch(t, dir, "complete.ogg")
	touch(t, dir, "chime.wav")  // a wav, but not our stem
	touch(t, dir, "sound.flac") // our stem, but a format we do not claim
	touch(t, dir, "sounds.wav") // nearly our name, wrong stem

	if got, ok := Find(dir); ok {
		t.Errorf("Find = %q, true; want only %s{.wav,.mp3} to count", got, Base)
	}
}

func TestFindReportsNoSoundInAnEmptyDirectory(t *testing.T) {
	if got, ok := Find(t.TempDir()); ok {
		t.Errorf("Find = %q, true; want no sound at all", got)
	}
}

// A directory named like the sound is not a sound, and trying to play it would
// hand a directory to the player and fail.
func TestFindIgnoresADirectoryNamedLikeTheSound(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, DefaultFile), 0o755); err != nil {
		t.Fatal(err)
	}

	if got, ok := Find(dir); ok {
		t.Errorf("Find = %q, true; want a directory to be ignored", got)
	}
}

// Play must run whichever player is actually installed, and pass it the sound.
func TestPlayRunsTheFirstAvailablePlayerWithTheSound(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, DefaultFile)

	s := &stub{available: map[string]bool{"aplay": true, "mpv": true}}
	s.install(t)

	Play(dir)

	if len(s.ran) != 1 {
		t.Fatalf("ran %v, want exactly one player", s.ran)
	}
	if s.ran[0] != "aplay" {
		t.Errorf("ran %q, want aplay, the first available player", s.ran[0])
	}
	if len(s.ranArgs[0]) == 0 || s.ranArgs[0][len(s.ranArgs[0])-1] != filepath.Join(dir, DefaultFile) {
		t.Errorf("args = %v, want the sound path last", s.ranArgs[0])
	}
}

// A WAV-only player must not be handed an MP3. aplay would exit non-zero and
// the chime would be silently lost, so Play has to skip players that cannot
// play the file's format rather than pretending every player handles every file.
func TestPlaySkipsAPlayerThatCannotHandleTheFormat(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "sound.mp3")

	s := &stub{available: map[string]bool{"aplay": true}}
	s.install(t)

	Play(dir)

	if len(s.ran) != 0 {
		t.Errorf("ran %v, want the wav-only player to be skipped", s.ran)
	}
}

// When a format-capable player is installed alongside a wav-only one, the
// capable player is the one that runs.
func TestPlayPicksAPlayerThatHandlesTheMP3(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "sound.mp3")

	s := &stub{available: map[string]bool{"aplay": true, "mpg123": true}}
	s.install(t)

	Play(dir)

	if len(s.ran) != 1 || s.ran[0] != "mpg123" {
		t.Fatalf("ran %v, want only mpg123", s.ran)
	}
	if last := s.ranArgs[0][len(s.ranArgs[0])-1]; last != filepath.Join(dir, "sound.mp3") {
		t.Errorf("args = %v, want the mp3 path last", s.ranArgs[0])
	}
}

// The reverse also holds: an mp3-only player must not be handed a WAV.
func TestPlayDoesNotHandAWAVToAnMP3OnlyPlayer(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, DefaultFile)

	s := &stub{available: map[string]bool{"mpg123": true}}
	s.install(t)

	Play(dir)

	if len(s.ran) != 0 {
		t.Errorf("ran %v, want the mp3-only player to be skipped for a wav", s.ran)
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
	touch(t, dir, DefaultFile)

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
	touch(t, dir, DefaultFile)

	s := &stub{available: map[string]bool{"afplay": true}, fail: true}
	s.install(t)

	Play(dir) // must not panic

	if len(s.ran) != 1 {
		t.Errorf("ran %d players, want the failing one tried once", len(s.ran))
	}
}
