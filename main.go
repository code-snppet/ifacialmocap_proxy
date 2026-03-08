package main

import (
	"flag"
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	lip := flag.String("ip", "", "Listening IP")
	lport := flag.Int("port", 0, "Listening port")
	flag.Parse()
	model := initialModel(*lip, *lport)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatalln("Fatal error: " + err.Error())
	}
}
