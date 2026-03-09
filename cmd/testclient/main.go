package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"
)

const handshake = "iFacialMocap_sahuasouryya9218sauhuiayeta91555dy3719"

func main() {
	relayAddr := flag.String("relay", "127.0.0.1:49983", "Relay address (host:port)")
	localPort := flag.Int("port", 0, "Local port to bind (0 = random)")
	passiveListenAddr := flag.String("passive-listen", "", "Passive listen address (host:port)")
	interval := flag.Duration("ping", 2*time.Second, "Handshake re-send interval (0 to send once)")
	flag.Parse()

	raddr, err := net.ResolveUDPAddr("udp4", *relayAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad relay address: %v\n", err)
		os.Exit(1)
	}

	var laddr *net.UDPAddr
	if *passiveListenAddr != "" {
		laddr, err = net.ResolveUDPAddr("udp4", *passiveListenAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad passive listen address: %v\n", err)
			os.Exit(1)
		}
	} else {
		laddr = &net.UDPAddr{IP: net.IPv4zero, Port: *localPort}
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("test client listening on %s\n", conn.LocalAddr())
	fmt.Printf("sending handshake to relay at %s\n\n", raddr)

	shouldSendHandshake := *passiveListenAddr == ""
	if shouldSendHandshake {
		if _, err := conn.WriteToUDP([]byte(handshake), raddr); err != nil {
			fmt.Fprintf(os.Stderr, "handshake send: %v\n", err)
			os.Exit(1)
		}
	}

	if shouldSendHandshake && *interval > 0 {
		go func() {
			tick := time.NewTicker(*interval)
			defer tick.Stop()
			for range tick.C {
				conn.WriteToUDP([]byte(handshake), raddr)
			}
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Println("\nshutting down")
		conn.Close()
		os.Exit(0)
	}()

	buf := make([]byte, 8192)
	count := 0
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			return
		}
		count++
		data := string(buf[:n])

		display := data
		if len(display) > 120 {
			display = display[:120] + "..."
		}

		fmt.Printf("#%d [%s] from %s (%d bytes)\n  %s\n",
			count, time.Now().Format("15:04:05.000"), from, n, display)
	}
}
