package main

import (
	_ "embed"
	"fmt"

	"github.com/gen2brain/beeep"
)

//go:embed info.png
var icon []byte

// INFO: the icon is not working
func Notify(session string) {
	err := beeep.Notify("Toki", fmt.Sprintf("%s session completed", session), icon)
	if err != nil {
		panic(err)
	}
}
