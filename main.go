package main

import (
	"flag"
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	lport := flag.Int("port", IFM_PORT, "Listening port")
	flag.Parse()
	model := initialModel(*lport)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatalln("Fatal error: " + err.Error())
	}
}
