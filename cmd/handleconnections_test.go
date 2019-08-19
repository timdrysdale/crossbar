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

	messagesToDistribute := make(chan message, 10) //TODO make buffer length configurable
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

	listen = fmt.Sprintf("ws://127.0.0.1:8099") //%v/", port)

	host, err = url.Parse(listen)

	wg.Add(3)
	//func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message)
	go HandleConnections(closed, &wg, clientActionsChan, messagesToDistribute)

	//func HandleMessages(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, messagesChan <-chan message)
	go HandleMessages(closed, &wg, &topics, messagesToDistribute)

	//func HandleClients(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, clientActionsChan chan clientAction)
	go HandleClients(closed, &wg, &topics, clientActionsChan)

	//wait for server to be up?
	time.Sleep(10 * time.Second)

	//send := fmt.Sprintf("%vin/0987oihlkjfhasdfgkjh", listen)

	//clientSendJSON(t, send)

	//go clientReceiveJSON(t, send)

	//time.Sleep(1 * time.Second)
}

func clientSendJSON(t *testing.T, url string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Printf("ClientSendJSON ctx %v", ctx)
	c, _, err := websocket.Dial(ctx, url, websocket.DialOptions{})
	if err != nil {
		t.Errorf("%v", err)
	}
	defer c.Close(websocket.StatusInternalError, "clientSendJSON:the sky is falling")

	err = wsjson.Write(ctx, c, map[string]int{
		"i": 123,
	})
	if err != nil {
		t.Errorf("%v", err)
	}
	time.Sleep(time.Second)

	c.Close(websocket.StatusNormalClosure, "")

}

func clientReceiveJSON(t *testing.T, url string) {
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

	fmt.Printf("received: %v\n", v)
	time.Sleep(time.Second)
	c.Close(websocket.StatusNormalClosure, "")

}
