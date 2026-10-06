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
)

// Name is the one filename toki looks for in the config directory. A single
// fixed name means there is no precedence to learn and nothing to guess.
const Name = "sound.wav"

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
	// args builds the command line for a sound file, because the flags differ
	// between players and one of them is not even a flag-based tool.
	args func(file string) []string
}

// players are tried in order, and the first whose program is installed wins.
// The list mixes platforms on purpose: only the ones present in PATH can match,
// so a macOS machine picks afplay, a Linux desktop picks paplay or aplay, and a
// minimal system falls through to whatever it has, if anything.
var players = []player{
	{name: "afplay", args: func(f string) []string { return []string{f} }},
	{name: "paplay", args: func(f string) []string { return []string{f} }},
	{name: "aplay", args: func(f string) []string { return []string{"-q", f} }},
	{name: "pw-play", args: func(f string) []string { return []string{f} }},
	{name: "mpg123", args: func(f string) []string { return []string{"-q", f} }},
	{name: "mpv", args: func(f string) []string { return []string{"--no-video", "--really-quiet", f} }},
	{name: "ffplay", args: func(f string) []string { return []string{"-nodisp", "-autoexit", "-loglevel", "quiet", f} }},
	{name: "mplayer", args: func(f string) []string { return []string{"-really-quiet", f} }},
	{
		name: "powershell",
		args: func(f string) []string {
			// PlaySync blocks until the sound finishes, which is what a
			// fire-and-forget goroutine wants: the process stays alive for the
			// duration of the sound and then exits.
			return []string{"-NoProfile", "-Command", "(New-Object Media.SoundPlayer '" + f + "').PlaySync()"}
		},
	},
}

// lookPath and run are indirected so tests can choose the player and see the
// command line without touching PATH or the speakers.
var (
	lookPath = exec.LookPath
	run      = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
)

// Find returns the path of the completion sound in dir, and whether one exists.
// A directory that happens to share the sound's name is not a sound.
func Find(dir string) (string, bool) {
	path := filepath.Join(dir, Name)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path, true
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

	for _, p := range players {
		if _, err := lookPath(p.name); err != nil {
			continue
		}
		// Run the first player that exists and stop. Trying a second one after
		// a failure risks playing the sound twice on a system where the first
		// player worked but exited non-zero.
		_ = run(p.name, p.args(file)...)
		return
	}
}
