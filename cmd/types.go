package cmd

import (
	"sync"

	"github.com/gobwas/ws"
)

// messages will be wrapped in this struct for muxing
type message struct {
	sender clientDetails
	op     ws.OpCode
	data   []byte //text data are converted to/from bytes as needed
}

type clientDetails struct {
	name         string
	topic        string
	messagesChan chan message
}

// requests to add or delete subscribers are represented by this struct
type clientAction struct {
	action clientActionType
	client clientDetails
}

// userActionType represents the type of of action requested
type clientActionType int

// clientActionType constants
const (
	clientAdd clientActionType = iota
	clientDelete
)

type topicDirectory struct {
	sync.Mutex
	directory map[string][]clientDetails
}

// gobwas/ws
type readClientDataReturns struct {
	msg []byte
	op  ws.OpCode
	err error
}
