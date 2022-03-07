package server

func Run(listenAddr string, peerAddrs []string) {
	println("server starts")
	server := CreateServer(listenAddr, peerAddrs)
	server.MainLoop()
}
