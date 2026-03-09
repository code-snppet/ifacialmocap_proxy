package main

import (
	"flag"
	"log"

	"codesnppet.dev/ifmproxy/logger"
	"codesnppet.dev/ifmproxy/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	lip := flag.String("ip", "", "Listening IP")
	lport := flag.Int("port", 0, "Listening port")
	flag.Parse()

	logger := logger.NewLogger(1000)
	log.SetOutput(logger)
	log.SetFlags(0)

	model := tui.InitialModel(*lip, *lport, logger)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatalln("Fatal error: " + err.Error())
	}
}
