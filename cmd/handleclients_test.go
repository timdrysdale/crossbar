package cmd

import (
	"testing"

	"github.com/google/uuid"
)

func TestAddClientToTopic(t *testing.T) {

	var topics topicDirectory
	topics.directory = make(map[string][]clientDetails)

	client1 := randomClient()
	client2 := randomClient()
	client3 := randomClientForTopic(client2.topic)
	client4 := randomClientForTopic(client2.topic)

	addClientToTopic(&topics, client1)
	addClientToTopic(&topics, client2)
	addClientToTopic(&topics, client3)
	addClientToTopic(&topics, client4)

	clientList := []clientDetails{client1, client2, client3, client4}
	clientShouldExist := []bool{true, true, true, true}

	for i := range clientList {
		if clientExists(&topics, clientList[i]) != clientShouldExist[i] {
			t.Errorf("addClientToTopic: client %v has WRONG existence status, should be %v\n", i, clientShouldExist[i])
		}
	}

	deleteClientFromTopic(&topics, client2)
	deleteClientFromTopic(&topics, client4)

	clientShouldExist = []bool{true, false, true, false}

	for i := range clientList {
		if clientExists(&topics, clientList[i]) != clientShouldExist[i] {
			t.Errorf("deleteClientFromTopic(): client %v has WRONG existence status, should be %v\n", i, clientShouldExist[i])
		}
	}

}

func randomClient() clientDetails {
	return clientDetails{uuid.New().String(), uuid.New().String(), make(chan message)}
}

func randomClientForTopic(topic string) clientDetails {
	return clientDetails{uuid.New().String(), topic, make(chan message)}
}

func clientExists(topics *topicDirectory, client clientDetails) bool {

	topics.Lock()
	existingClients := topics.directory[client.topic]
	topics.Unlock()

	for _, existingClient := range existingClients {
		if client.name == existingClient.name {
			return true

		}
	}

	return false

}
