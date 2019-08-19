package cmd

import (
	"fmt"
	"sync"
)

func HandleMessages(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, messagesChan <-chan message) {
	defer wg.Done()

	for {
		select {
		case <-closed:
			return
		case message := <-messagesChan:
			distributeMessage(topics, message)
		}
	}
}

func distributeMessage(topics *topicDirectory, message message) {

	// unsubscribing client would close channel so lock throughout
	topics.Lock()

	distributionList := topics.directory[message.sender.topic]

	// assuming buffered messageChans, all writes should succeed immediately
	for _, destination := range distributionList {
		//don't send to sender
		if destination.name != message.sender.name {
			//non-blocking write to chan - skips if can't write
			select {
			case destination.messagesChan <- message:
			default:
				fmt.Printf("Warn: not sending message to %v (%v)\n", destination, message) //TODO log this "properly"
			}
		}
	}

	topics.Unlock()
}
