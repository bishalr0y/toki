package app

import (
	"os"
	"testing"
)

// TestMain clears XDG_CONFIG_HOME for the whole package.
//
// Dir honours the variable when it is set, and tests that stub HOME expect the
// config to land in the temporary directory they named. A machine that has
// XDG_CONFIG_HOME set — which a CI runner does — silently sends every one of them
// to the real config directory instead, where they read whatever is there and
// write to it. That is not a test failure to be worked around per case: it means
// the suite reaches outside itself, so it is closed off here once for the package.
//
// Setting it to empty rather than unsetting is what the tests already do, and
// Dir treats an empty value as unset.
func TestMain(m *testing.M) {
	os.Setenv("XDG_CONFIG_HOME", "")

	os.Exit(m.Run())
}
