package cmd

import (
	"sync"

	"github.com/eclesh/welford"
)

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
	mt  int
	err error
}

type summaryStats struct {
	topic map[string]topicStats
}

type topicStats struct {
	audience *welford.Stats
	size     *welford.Stats
	rx       map[string]int
}

type messageStats struct {
	topic string
	rx    []string
	size  int
}
