package app

// Key bindings, in one place so the help line can never drift from the keys
// that actually work.
const (
	keyEnter  = "enter"
	keySpace  = "space"
	keyUp     = "up"
	keyDown   = "down"
	keyEsc    = "esc"
	keyQuit   = "q"
	keyCtrlC  = "ctrl+c"
	keySkip   = "s"
	keyDigits = "123456789"
)

func isQuit(k string) bool { return k == keyQuit || k == keyCtrlC }

func isUp(k string) bool { return k == keyUp || k == "k" }

func isDown(k string) bool { return k == keyDown || k == "j" }

// digitIndex maps a number key to a zero-based row index, reporting whether the
// key was a digit at all.
func digitIndex(k string, count int) (int, bool) {
	if len(k) != 1 || k[0] < '1' || k[0] > '9' {
		return 0, false
	}

	idx := int(k[0] - '1')
	if idx >= count {
		return 0, false
	}
	return idx, true
}
