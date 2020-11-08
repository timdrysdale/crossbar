package cmd

import (
	"sync"
)

func crossbar(config Config, closed chan struct{}, parentwg *sync.WaitGroup) {

	var wg sync.WaitGroup

	messagesToDistribute := make(chan message, 10) //TODO make buffer length configurable

	var topics topicDirectory

	topics.directory = make(map[string][]clientDetails)

	clientActionsChan := make(chan clientAction)

	wg.Add(2)

	go HandleConnections(closed, &wg, clientActionsChan, messagesToDistribute, config)

	go HandleClients(closed, &wg, &topics, clientActionsChan)

	wg.Wait()

	parentwg.Done()

}
