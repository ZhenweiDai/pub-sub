package main

import (
	"flag"
	"strings"

	"github.com/ninomiyx/pub-sub/client"
	"github.com/ninomiyx/pub-sub/domain"
	"github.com/ninomiyx/pub-sub/server"
)

var (
	mode       = flag.String("mode", "server", "either <server> or <client>")
	serverAddr = flag.String("server-addr", "127.0.0.1:5001", "server listener address")
	peerAddrs  = flag.String("peer-addrs", "127.0.0.1:5001,127.0.0.1:5002,127.0.0.1:5003", "comma separated list of peer addresses")
)

func main() {
	flag.Parse()
	domain.MessageRegistry()
	switch *mode {
	case "server":
		server.Run(*serverAddr, strings.Split(*peerAddrs, ","))
	case "client":
		client.Run(*serverAddr, strings.Split(*peerAddrs, ","))
	}
}
