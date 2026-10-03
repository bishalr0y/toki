package notify

import (
	_ "embed"

	"github.com/gen2brain/beeep"
)

//go:embed info.png
var icon []byte

func Notify(session string) error {
	return beeep.Notify("Toki", session+" session completed", icon)
}
