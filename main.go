package main

import (
	"os"

	"github.com/deckplane/cli/cmd"
	"github.com/deckplane/cli/internal/ui"
)

func main() {
	if err := cmd.NewApp().Run(os.Args[1:]); err != nil {
		ui.PrintError(err)
		os.Exit(1)
	}
}
