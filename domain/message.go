package domain

import "encoding/gob"

type MessageType int

const (
	Normal MessageType = iota
	Subscribe
	Unsubscribe
)

// the pub sub message
type Message struct {
	// type of the message
	Type MessageType
	// id of the message
	Id int64
	// topic of the message
	Topic string
	// the message
	Content string
}

type HandshakeMessageType int

const (
	Election HandshakeMessageType = iota
	ElectionOK
	Coordinator
	HelloFromClient
	HelloFromServerPeer
)

type HandshakeMessage struct {
	Type HandshakeMessageType
	Addr string
}

func MessageRegistry() {
	gob.Register(HandshakeMessage{})
	gob.Register(Message{})
}
