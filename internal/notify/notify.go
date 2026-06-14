package notify

import (
	_ "embed"
	"fmt"

	"github.com/gen2brain/beeep"
)

//go:embed info.png
var icon []byte

func Notify(session string) error {
	return beeep.Notify("Toki", fmt.Sprintf("%s session completed", session), icon)
}
