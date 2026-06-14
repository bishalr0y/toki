# toki (とき)

A minimal Pomodoro timer for the terminal.

## Features

- Terminal-based Pomodoro sessions with live countdown
- Configurable focus/break durations via YAML
- Desktop notifications when sessions complete
- Color-coded UI

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

Select a timer split from the menu. Press `q` to quit.

## Configuration

Config is stored at `~/.config/toki/config.yaml`:

```yaml
timers:
  - name: Standard
    focus: 25
    break: 5
  - name: Long Focus
    focus: 50
    break: 10
```

## Development

```bash
just test    # Run tests
just lint    # Run linter
just run     # Build and run
```

## TODO

- [ ] Store completed work sessions in BadgerDB
