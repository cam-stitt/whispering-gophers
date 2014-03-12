// Whispering Gophers is a simple whispernet written in Go and based off of
// Google's excellent code lab: https://code.google.com/p/whispering-gophers/
package main

import (
	"flag"
	"log"

	"github.com/pdxgo/whispering-gophers/terminal"
	"github.com/pdxgo/whispering-gophers/http"
	"github.com/pdxgo/whispering-gophers/util"
)

var (
	peerAddr = flag.String("peer", "", "peer host:port")
	bindPort = flag.Int("port", 55555, "port to bind to")
	selfNick = flag.String("nick", "Anonymous Coward", "nickname")
	httpAddr = flag.String("http", "localhost:8888", "local http interface")
)

func main() {
	flag.Parse()

	terminal.PeerAddr = *peerAddr
	terminal.BindPort = *bindPort
	terminal.SelfNick = *selfNick

	l, err := util.ListenWithPort(*bindPort)
	if err != nil {
		log.Fatal(err)
	}
	terminal.Self = l.Addr().String()
	if terminal.SelfNick == "" {
		log.Println("Listening on", terminal.Self)
	} else {
		log.Printf("Listening on %s as nick %s", terminal.Self, terminal.SelfNick)
	}

	go terminal.DiscoveryListen()
	go terminal.DiscoveryClient()
	go http.Serve(*httpAddr, terminal.Peers.Get)

	if *peerAddr != "" {
		go terminal.Dial(*peerAddr)
	} else {
		log.Println("No -peer specified. Waiting to receive discovery packets.")
	}
	go terminal.ReadInput()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatalf("Unable to accept connection: %v", err)
		}
		go terminal.Serve(c)
	}
}
