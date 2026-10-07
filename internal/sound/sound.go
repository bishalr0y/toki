// Package sound plays a completion sound from toki's config directory.
//
// It is deliberately best-effort: playback is a courtesy, not a feature toki
// depends on, so a missing sound, a machine with no audio player, or a player
// that fails are all silent no-ops rather than errors to report.
package sound

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Base is the one name toki looks for in the config directory. The extension
// only says which format the file is in, so there is still a single name to
// remember; there is no list of alternatives to choose between.
const Base = "sound"

// DefaultFile is the sound toki writes beside a freshly created config. It is
// the WAV of the embedded chime, which every supported player can play.
const DefaultFile = Base + ".wav"

// formats are the extensions toki recognises, in preference order. WAV comes
// first because it is what toki ships and what every player understands; MP3 is
// the one other format people commonly have lying around.
var formats = []string{".wav", ".mp3"}

//go:embed default.wav
var defaultSound []byte

// Default returns the built-in completion sound, a short chime embedded in the
// binary so a fresh install makes a sound without the user finding one. It is
// written to the config directory when the default config is created, and only
// then, so deleting it turns the sound off for good.
func Default() []byte { return defaultSound }

// player is one way to play a file on some systems.
type player struct {
	name string
	// formats lists the extensions this player can play; a nil slice means it
	// can play anything. The list is not decoration: the players do not all
	// handle the same formats, and handing an MP3 to a WAV-only player such as
	// aplay is a silent failure, not a fallback.
	formats []string
	// args builds the command line for a sound file, because the flags differ
	// between players and one of them is not even a flag-based tool.
	args func(file string) []string
}

// plays reports whether the player can be expected to handle a file with the
// given extension.
func (p player) plays(ext string) bool {
	if len(p.formats) == 0 {
		return true
	}
	return slices.Contains(p.formats, ext)
}

// players are tried in order, and the first installed one that can play the
// file's format wins. The list mixes platforms on purpose: only the ones
// present in PATH can match, so a macOS machine picks afplay, a Linux desktop
// picks paplay or aplay, and a minimal system falls through to whatever it has,
// if anything.
var players = []player{
	{name: "afplay", args: func(f string) []string { return []string{f} }},
	{name: "paplay", formats: wavOnly, args: func(f string) []string { return []string{f} }},
	{name: "aplay", formats: wavOnly, args: func(f string) []string { return []string{"-q", f} }},
	{name: "pw-play", formats: wavOnly, args: func(f string) []string { return []string{f} }},
	{name: "mpg123", formats: mp3Only, args: func(f string) []string { return []string{"-q", f} }},
	{name: "mpv", args: func(f string) []string { return []string{"--no-video", "--really-quiet", f} }},
	{name: "ffplay", args: func(f string) []string { return []string{"-nodisp", "-autoexit", "-loglevel", "quiet", f} }},
	{name: "mplayer", args: func(f string) []string { return []string{"-really-quiet", f} }},
	{
		name:    "powershell",
		formats: wavOnly,
		args: func(f string) []string {
			// PlaySync blocks until the sound finishes, which is what a
			// fire-and-forget goroutine wants: the process stays alive for the
			// duration of the sound and then exits.
			return []string{"-NoProfile", "-Command", "(New-Object Media.SoundPlayer '" + f + "').PlaySync()"}
		},
	},
}

// The players do not all handle all formats. These two lists say what the ones
// that are fussier can play; an empty player.formats means "anything".
var (
	wavOnly = []string{".wav"}
	mp3Only = []string{".mp3"}
)

// lookPath and run are indirected so tests can choose the player and see the
// command line without touching PATH or the speakers.
var (
	lookPath = exec.LookPath
	run      = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
)

// Find returns the path of the completion sound in dir, and whether one exists.
// The sound is Base with one of the recognised extensions; the first format in
// preference order that is actually present wins. A directory that happens to
// share the name is not a sound.
func Find(dir string) (string, bool) {
	for _, ext := range formats {
		path := filepath.Join(dir, Base+ext)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// Play plays the completion sound in dir, if there is one and a player can be
// found for it. Every failure is swallowed: there is nothing the user asked to
// be told about, so there is nothing to say.
func Play(dir string) {
	file, ok := Find(dir)
	if !ok {
		return
	}

	ext := strings.ToLower(filepath.Ext(file))

	for _, p := range players {
		if !p.plays(ext) {
			continue
		}
		if _, err := lookPath(p.name); err != nil {
			continue
		}
		// Run the first player that exists and handles this format, then stop.
		// Trying a second one after a failure risks playing the sound twice on
		// a system where the first player worked but exited non-zero.
		_ = run(p.name, p.args(file)...)
		return
	}
}
