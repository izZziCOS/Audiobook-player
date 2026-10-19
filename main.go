package main

import (
    _ "embed"

    "fyne.io/fyne/v2"

    "github.com/izzzicos/audiobook-player/ui"
)

//go:embed Icon.png
var icon []byte

func main() {
    playerUI := ui.NewUI(fyne.NewStaticResource("Icon.png", icon))
    playerUI.Run()
}
