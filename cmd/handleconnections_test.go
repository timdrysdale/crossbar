package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message)

func TestHandleConnections(t *testing.T) {

	var wg sync.WaitGroup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGINT)
	signal.Notify(c, syscall.SIGTERM)

	messagesToDistribute := make(chan message) //TODO make buffer length configurable
	var topics topicDirectory
	topics.directory = make(map[string][]clientDetails)
	clientActionsChan := make(chan clientAction)
	closed := make(chan struct{})
	go func() {
		for _ = range c {

			close(closed)
			wg.Wait()
			os.Exit(1)

		}
	}()

	bufferSize = 32798

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Errorf("Error getting free port %v", err)
	}
	fmt.Printf("port: %v\n", port)

	listen = fmt.Sprintf("ws://127.0.0.1:%v", port)

	host, err = url.Parse(listen)

	wg.Add(3)
	//func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message)
	go HandleConnections(closed, &wg, clientActionsChan, messagesToDistribute)

	//func HandleMessages(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, messagesChan <-chan message)
	go HandleMessages(closed, &wg, &topics, messagesToDistribute)

	//func HandleClients(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, clientActionsChan chan clientAction)
	go HandleClients(closed, &wg, &topics, clientActionsChan)

	//wait for server to be up?
	time.Sleep(10 * time.Millisecond)

	topic1 := "ws://127.0.0.1:8097/topic1" //fmt.Sprintf("%vin/stream01", listen)
	topic2 := "ws://127.0.0.1:8097/topic2" //fmt.Sprintf("%vin/stream02", listen)
	i1 := 1
	i2 := 2

	go clientReceiveJSON(t, topic1, i1)
	go clientReceiveJSON(t, topic1, i1)
	go clientReceiveJSON(t, topic1, i1)
	go clientReceiveJSON(t, topic2, i2)
	go clientReceiveJSON(t, topic2, i2)
	go clientReceiveJSON(t, topic2, i2)

	time.Sleep(10 * time.Millisecond)

	clientSendJSON(t, topic1, i1)
	clientSendJSON(t, topic2, i2) //cause an error
	time.Sleep(10 * time.Millisecond)
}

// This example dials a server, writes a single JSON message and then

func clientSendJSON(t *testing.T, url string, i int) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, _, err := websocket.Dial(ctx, url, websocket.DialOptions{}) //"ws://127.0.0.1:8097"

	if err != nil {
		t.Errorf("clientSendJson dial error: %v\n", err)
	}

	defer c.Close(websocket.StatusInternalError, "clientSendJSON:the sky is falling")

	err = wsjson.Write(ctx, c, map[string]int{
		"i": i,
	})

	if err != nil {
		t.Errorf("clientSendJSON Write Error%v\n", err)
	}
	time.Sleep(time.Second)

	c.Close(websocket.StatusNormalClosure, "")

}

func clientReceiveJSON(t *testing.T, url string, i int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	c, _, err := websocket.Dial(ctx, url, websocket.DialOptions{})
	if err != nil {
		t.Errorf("%v", err)
	}
	defer c.Close(websocket.StatusInternalError, "ClientReceiveJSON: the sky is falling")
	v := map[string]int{}

	err = wsjson.Read(ctx, c, &v)
	if err != nil {
		t.Errorf("%v", err)
	}

	if v["i"] != i {
		t.Errorf("Expected {i:%v}, got %v\n", i, v["i"])
	} else {
		fmt.Printf("%v got %v\n", url, i)
	}

	fmt.Printf("received: %v\n", v)

	time.Sleep(time.Second)
	c.Close(websocket.StatusNormalClosure, "")

}
