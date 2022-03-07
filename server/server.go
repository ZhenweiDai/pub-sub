package server

import (
	"encoding/gob"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/ninomiyx/pub-sub/domain"
)

type ServerState int

const (
	Leader ServerState = iota
	Follower
	Waiting
)

type PeerConn struct {
	conn net.Conn
	enc  *gob.Encoder
}

// the config for server
type Server struct {
	// the address server listens on
	listener net.Listener
	// local address
	localAddr string
	// the addresses of peers
	peerAddrs []string

	// the channel for message input
	localMessageIngest chan *domain.Message

	// the channel for message sending to leader
	sendToLeader chan *domain.Message
	// the channel for message receiving from leader
	receiveFromLeader chan *domain.Message

	// client connections
	clientSubscriptions map[string]([]*PeerConn)
	// peer connections
	peerConnections []*PeerConn

	// the connection to leader
	leaderConn net.Conn
	// the string address of leader
	leaderAddr string
	// the state of the server
	state ServerState
	// the lock for modify server data
	mu sync.Mutex
	// next message id
	nextMsgId int64
	// last start waiting time
	lastStartWaitingTime time.Time
	// election timeout
	electionTimeout time.Duration
	// waiting timeout
	waitingTimeout time.Duration
}

func CreateServer(listenAddr string, peerAddrs []string) *Server {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}

	// make sure peer addresses contains local address and sorted
	peerContainsLocalAddr := false
	for _, addr := range peerAddrs {
		if addr == listenAddr {
			peerContainsLocalAddr = true
			break
		}
	}
	if !peerContainsLocalAddr {
		peerAddrs = append(peerAddrs, listenAddr)
	}
	sort.Strings(peerAddrs)

	fmt.Println("Sorted peers: ", peerAddrs)

	server := &Server{
		listener:             ln,
		localAddr:            listenAddr,
		peerAddrs:            peerAddrs,
		localMessageIngest:   make(chan *domain.Message),
		sendToLeader:         make(chan *domain.Message),
		receiveFromLeader:    make(chan *domain.Message),
		clientSubscriptions:  make(map[string]([]*PeerConn)),
		peerConnections:      make([]*PeerConn, 0),
		leaderConn:           nil,
		state:                Waiting,
		mu:                   sync.Mutex{},
		nextMsgId:            0, // TODO this should be in the disk for persistence in the cluster
		lastStartWaitingTime: time.UnixMilli(0),
		waitingTimeout:       time.Second * 1,
		electionTimeout:      time.Second * 1,
	}

	go server.acceptConn()
	go server.processMessage()

	return server
}

// assign global message id
func (s *Server) processMessage() {
	filterOutNilEnc := func(peers []*PeerConn) []*PeerConn {
		result := make([]*PeerConn, 0)
		for _, peer := range peers {
			if peer.enc != nil {
				result = append(result, peer)
			} else {
				if (peer.conn).Close() != nil {
					fmt.Println("error closing connection")
				}
			}
		}
		return result
	}

	cleanUpDeadConnections := func(peerNeedsCleanup bool, clientNeedsCleanup bool, topic string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if peerNeedsCleanup {
			s.peerConnections = filterOutNilEnc(s.peerConnections)
		}
		if clientNeedsCleanup {
			s.clientSubscriptions[topic] = filterOutNilEnc(s.clientSubscriptions[topic])
		}
	}

	sendMessage := func(msg *domain.Message) {
		fmt.Printf("Sending message %v\n", msg)
		peerNeedsCleanup := false
		if s.state == Leader {
			for _, peerConn := range s.peerConnections {
				err := peerConn.enc.Encode(msg)
				if err != nil {
					fmt.Println("error sending message to peer")
					fmt.Println(err)
					peerNeedsCleanup = true
					peerConn.enc = nil
				}
			}
		}

		clientNeedsCleanup := false
		// fmt.Printf("Sending message %v to subscriptions %v\n", msg, s.clientSubscriptions)
		if clients, ok := s.clientSubscriptions[msg.Topic]; ok {
			// fmt.Printf("Sending message %v to clients %v\n", msg, clients)
			for _, clientConn := range clients {
				// fmt.Printf("Sending message %v to client %v\n", msg, clientConn.conn.RemoteAddr())
				err := clientConn.enc.Encode(msg)
				if err != nil {
					fmt.Println("error sending message to client")
					fmt.Println(err)
					clientNeedsCleanup = true
					(clientConn.conn).Close()
					clientConn.conn = nil
				}
			}
		}
		if peerNeedsCleanup || clientNeedsCleanup {
			cleanUpDeadConnections(peerNeedsCleanup, clientNeedsCleanup, msg.Topic)
		}
	}

	for {
		select {
		case msg := <-s.localMessageIngest:
			if msg.Type != domain.Normal {
				fmt.Println("unexpected message type from local message ingest")
				fmt.Println(msg.Type)
				continue
			}
			if s.state == Leader {
				msg.Id = s.nextMsgId
				s.nextMsgId++
				sendMessage(msg)
			} else {
				s.sendToLeader <- msg
			}
		case msg := <-s.receiveFromLeader:
			if msg.Type != domain.Normal {
				fmt.Println("unexpected message type from leader")
				fmt.Println(msg.Type)
				continue
			}
			if s.state != Follower {
				fmt.Println("unexpected state when receive from leader")
				fmt.Println(s.state)
				continue
			}
			sendMessage(msg)
		}
	}
}

func (s *Server) MainLoop() {
	for {
		s.election()
		time.Sleep(s.waitingTimeout)
	}
}

func (s *Server) setNewLeaderConnection(leaderConn net.Conn, leaderAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.leaderConn != nil {
		s.leaderConn.Close()
	}
	s.leaderAddr = leaderAddr
	s.leaderConn = leaderConn
}

func (s *Server) closeLeaderConnection() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.leaderConn != nil {
		fmt.Println("Closing leader connection")
		s.leaderConn.Close()
	}
	s.leaderConn = nil
}

func (s *Server) connectToLeader(leaderAddr string) {
	fmt.Println("connecting")
	// connect to leader
	leadConn, err := net.Dial("tcp", leaderAddr)
	if err != nil {
		fmt.Println("error connecting to leader")
		fmt.Println(err)
		return
	}
	s.setNewLeaderConnection(leadConn, leaderAddr)
	defer s.closeLeaderConnection()
	encoder := gob.NewEncoder(leadConn)
	decoder := gob.NewDecoder(leadConn)
	initMsg := &domain.HandshakeMessage{
		Type: domain.HelloFromServerPeer,
	}
	err = encoder.Encode(initMsg)
	if err != nil {
		fmt.Println("error sending hello to leader")
		fmt.Println(err)
		return
	}

	go func() {
		defer s.closeLeaderConnection()
		for s.state == Follower {
			msg := <-s.sendToLeader
			err = encoder.Encode(msg)
			if err != nil {
				fmt.Println("error sending message to leader")
				fmt.Println(err)
				break
			}
		}
	}()

	for s.state == Follower {
		msg := &domain.Message{}
		err = decoder.Decode(msg)
		if err != nil {
			fmt.Println("error receiving message from leader")
			fmt.Println(err)
			break
		}
		s.receiveFromLeader <- msg
	}
}

func (s *Server) acceptConn() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			fmt.Println("error accepting connection")
			fmt.Println(err)
			continue
		}
		go s.handleConn(conn)
	}
}

func isPeerReturningOK(addr string, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		fmt.Println("error connecting to candidate" + addr)
		fmt.Println(err)
		return false
	}
	defer conn.Close()
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	initMsg := &domain.HandshakeMessage{
		Type: domain.Election,
	}
	encoder.Encode(initMsg)
	decoder.Decode(initMsg)
	if initMsg.Type == domain.ElectionOK {
		fmt.Println("Election OK from " + addr)
		return true
	}
	fmt.Println("Unexpected election response from " + addr)
	return false
}

func (s *Server) sendCoordinateMessageToPeer(peerAddr string) {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		fmt.Println("send coordinate message error, failed to connect peer" + peerAddr)
		fmt.Println(err)
		return
	}
	defer conn.Close()
	encoder := gob.NewEncoder(conn)
	initMsg := &domain.HandshakeMessage{
		Type: domain.Coordinator,
		Addr: s.localAddr,
	}
	encoder.Encode(initMsg) // send the message and close the connection
}

func (s *Server) election() {
	if s.leaderConn != nil {
		return
	}

	// handle the election
	// check if election has started, if not, start election
	if s.state == Waiting && time.Since(s.lastStartWaitingTime) < s.waitingTimeout {
		return
	}

	// find higher peers
	leaderCandidates := make([]string, 0)
	for _, addr := range s.peerAddrs {
		if addr == s.localAddr {
			break
		}
		leaderCandidates = append(leaderCandidates, addr)
	}

	// send to higher peer
	isOkChannel := make(chan bool)
	for _, peerAddr := range leaderCandidates {
		go func(addr string) {
			isOkChannel <- isPeerReturningOK(addr, s.electionTimeout)
		}(peerAddr)
	}

	// check if any of higher peer is returning OK
	for i := 0; i < len(leaderCandidates); i++ {
		if <-isOkChannel { // if any one of the peer return true, then waiting
			s.state = Waiting
			return
		}
	}

	// no higher peer reply election ok, become leader, send coordinator to all peers lower than itself
	// fmt.Println("become leader")
	s.state = Leader
	for i := len(leaderCandidates); i < len(s.peerAddrs); i++ {
		if s.peerAddrs[i] == s.localAddr {
			continue
		}
		go s.sendCoordinateMessageToPeer(s.peerAddrs[i])
	}
}

func (s *Server) subscribe(peerConn *PeerConn, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pre, ok := s.clientSubscriptions[topic]; ok {
		s.clientSubscriptions[topic] = append(pre, peerConn)
	} else {
		s.clientSubscriptions[topic] = []*PeerConn{peerConn}
	}
	// fmt.Printf("subscriptions: %v\n", s.clientSubscriptions)
}

func (s *Server) unsubscribe(peerConn *PeerConn, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pre, ok := s.clientSubscriptions[topic]; ok {
		next := make([]*PeerConn, 0)
		for _, conn := range pre {
			if conn != peerConn {
				next = append(next, conn)
			}
		}
		s.clientSubscriptions[topic] = next
	} else {
		fmt.Println("unsubscribe error, topic not found")
	}
}

func (s *Server) appendPeer(peerConn *PeerConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peerConnections = append(s.peerConnections, peerConn)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	peerConn := &PeerConn{
		conn: conn,
		enc:  encoder,
	}
	initMsg := &domain.HandshakeMessage{}
	decoder.Decode(initMsg)
	switch initMsg.Type {
	case domain.Election:
		fmt.Println("Election")
		// assuming the client is always connect to peer higher than itself
		// reply election ok
		encoder.Encode(&domain.HandshakeMessage{
			Type: domain.ElectionOK,
		})
		go s.election()
	case domain.ElectionOK:
		fmt.Println("Election OK is not expected")
	case domain.Coordinator:
		if s.state == Follower && initMsg.Addr == s.leaderAddr {
			// fmt.Println("Received coordinator message from the same leader")
		} else {
			fmt.Println("New leader: " + initMsg.Addr) // leader address
			s.state = Follower
			// connect to leader
			go s.connectToLeader(initMsg.Addr)
		}
	case domain.HelloFromClient:
		fmt.Println("Hello from client")
		for {
			msg := &domain.Message{}
			decoder.Decode(msg)
			switch {
			case msg.Type == domain.Normal:
				fmt.Printf("%s -> %s\n", msg.Topic, msg.Content)
				s.localMessageIngest <- msg
			case msg.Type == domain.Subscribe:
				fmt.Printf("subscribe %s\n", msg.Topic)
				s.subscribe(peerConn, msg.Topic)
			case msg.Type == domain.Unsubscribe:
				fmt.Printf("unsubscribe %s\n", msg.Topic)
				s.unsubscribe(peerConn, msg.Topic)
			}
		}
	case domain.HelloFromServerPeer:
		s.appendPeer(peerConn)
		for {
			msg := &domain.Message{}
			decoder.Decode(msg)
			s.localMessageIngest <- msg
		}
	}
}
