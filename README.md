# toki (とき)

A Pomodoro timer for the terminal.

## Features

- Full-screen timer built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- Real Pomodoro rhythm: several focus rounds, a short break between each, and a
  longer one after the last
- Pause, resume and skip phases without stopping
- Countdown that tracks the wall clock, so it stays accurate across a suspend
- A summary of what you actually focused, not what the plan called for
- Configurable splits via YAML
- A completion sound per phase, on by default but easy to swap or delete
- Live progress bar, and colour that respects the terminal's own palette

## Installation

```bash
go install github.com/bishalr0y/toki/cmd/toki@latest
```

Or build from source:

```bash
git clone https://github.com/bishalr0y/toki
cd toki
just build
./bin/toki
```

## Usage

```bash
toki              # start the interface and pick a split
toki --list       # print the configured splits and exit
toki --help       # show the help and exit
toki --version    # print the version and exit
```

Pick a split with `↑` `↓` and `enter`, or press its number to start it
immediately. `q` quits from anywhere; `esc` abandons a split without recording
it.

`toki --list` prints the splits as they will actually run, so it is the quickest
way to check that an edit to `config.yaml` took effect without launching
anything.

### Keys

| Key      | Menu           | Session          | Summary  |
| -------- | -------------- | ---------------- | -------- |
| `↑` `↓`  | move           | –                | –        |
| `1`–`9`  | start that one | –                | –        |
| `enter`  | start          | –                | back     |
| `space`  | –              | pause / resume   | –        |
| `s`      | –              | skip the phase   | –        |
| `esc`    | –              | back to the menu | –        |
| `q`      | quit           | quit             | back     |

## Configuration

Config is stored at `~/.config/toki/config.yaml`. Every split needs a name, a
focus length and a break length:

```yaml
timers:
  - name: Standard
    focus: 25
    break: 5
  - name: Long Focus
    focus: 50
    break: 10
```

A split also runs several focus rounds, with a long break after the last one.
Both are optional and default to four rounds and a fifteen minute rest:

```yaml
timers:
  - name: Standard
    focus: 25
    break: 5
    cycles: 4 # focus rounds before the long break
    long_break: 15 # minutes of rest after the last round
```

So the split above runs focus, break, focus, break, focus, break, focus, long
break — and is then finished, with a summary of what you actually worked.

## Completion sound

toki plays a short chime each time a phase ends, so a fresh install is not
silent. The sound ships in the binary and is written to your config directory the
first time toki runs:

```
~/.config/toki/sound.wav
```

The name is fixed; only the format is up to you. toki looks for `sound.wav` and
then `sound.mp3`, so a `.wav` wins if you keep both. To use your own sound,
replace the file — any `.wav` or `.mp3` you drop in works, as long as it keeps
the name. To turn the sound off, delete the file; toki will not put it back.

Playback is best-effort. toki hands the file to whichever player is installed
and can actually play that format — `afplay` on macOS, `paplay`/`aplay`/`pw-play`
for WAV, `mpg123` for MP3, and `mpv`/`ffplay`/`mplayer` for anything — and it
never blocks the countdown. If there is no sound file, or no player that can
handle it, the phase simply passes in silence; toki will not nag you about it.

## What it keeps

Nothing about your work. toki keeps no history: a finished split is summarised on
screen and then forgotten, so there is no record on your machine of when you
worked or for how long.

The only files it writes are its own setup, and only on the first run, when
neither exists yet: `config.yaml` and the default `sound.wav`. After that toki
reads them and leaves them alone — delete either one and it stays deleted.

If you want a record of your work kept, it belongs to something built to keep it:
a calendar, a time tracker, whatever you already use to account for your week.

## Development

```bash
just test    # Run tests
just lint    # Run linter
just run     # Build and run
```
