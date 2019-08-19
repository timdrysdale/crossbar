package cmd

import (
	"sync"

	"nhooyr.io/websocket"
)

//messages will be wrapped in this struct for muxing
type message struct {
	typ  websocket.MessageType
	data []byte //text data are converted to/from bytes as needed
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
