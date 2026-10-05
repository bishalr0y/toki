# toki (とき)

A Pomodoro timer for the terminal.

## Features

- Full-screen timer built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- Real Pomodoro rhythm: several focus rounds, a short break between each, and a
  longer one after the last
- Pause, resume and skip phases without stopping
- Countdown that tracks the wall clock, so it stays accurate across a suspend
- A summary of what you actually focused, not what the plan called for
- Finished splits recorded in plain JSON, with today's running total
- Configurable splits via YAML
- Desktop notifications per phase, with failures reported on screen
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
toki
```

Pick a split with `↑` `↓` and `enter`, or press its number to start it
immediately. `q` quits from anywhere; `esc` abandons a split without recording
it.

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
break — and is then finished, with a summary and a line in the history.

## History

Splits you finish are appended to `~/.config/toki/history.json`, next to the
config. It is plain JSON on purpose: readable, editable, and deletable without
this program.

```json
[
  {
    "split": "Standard",
    "startedAt": "2026-10-03T09:00:00Z",
    "endedAt": "2026-10-03T11:10:00Z",
    "focusSeconds": 5700,
    "rounds": 4
  }
]
```

`focusSeconds` counts time actually spent working, so it excludes breaks and
time spent paused. Abandoning a split records nothing.

## Development

```bash
just test    # Run tests
just lint    # Run linter
just run     # Build and run
```

## TODO

- [ ] Resume a split after an unexpected termination