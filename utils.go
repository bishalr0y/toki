package main

import (
	"os"
	"os/exec"
	"runtime"
)

func ClearConsole() {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Windows uses 'cls' via the command prompt
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		// Linux and macOS use the 'clear' command
		cmd = exec.Command("clear")
	}

	// Redirect the command's output to the standard output
	cmd.Stdout = os.Stdout
	cmd.Run()
}
