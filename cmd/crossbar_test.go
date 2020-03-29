package cmd

import (
	"bytes"
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"github.com/timdrysdale/reconws"
)

func TestCrossbarInOut(t *testing.T) {
	suppressLog()
	defer displayLog()
	// setup crossbar on local (free) port

	closed := make(chan struct{})
	var wg sync.WaitGroup

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}

	addr := ":" + strconv.Itoa(port)

	wg.Add(1)
	go crossbar(addr, closed, &wg)

	time.Sleep(10 * time.Millisecond)

	// set up test server and two clients

	ctx, cancel := context.WithCancel(context.Background())

	uc := "ws://127.0.0.1" + addr + "/out/some/location"
	us := "ws://127.0.0.1" + addr + "/in/some/location"

	c0 := reconws.New()
	c1 := reconws.New()
	s := reconws.New()

	go c0.Reconnect(ctx, uc)
	go c1.Reconnect(ctx, uc)
	go s.Reconnect(ctx, us)

	timeout := 50 * time.Millisecond

	time.Sleep(timeout)

	payload0 := []byte("Hello from client0")
	payload1 := []byte("Hello from client1")

	mtype := websocket.TextMessage

	c0.Out <- reconws.WsMessage{Data: payload0, Type: mtype}
	c1.Out <- reconws.WsMessage{Data: payload1, Type: mtype}

	expectNoMsg(s.In, timeout, t)  //should not see message from any client
	expectNoMsg(c0.In, timeout, t) //should not see message from other client
	expectNoMsg(c1.In, timeout, t) //should not see message from other client

	// broadcast from the server

	broadcast0 := []byte("First broadcast from server")
	broadcast1 := []byte("Second broadcast from server")

	s.Out <- reconws.WsMessage{Data: broadcast0, Type: websocket.BinaryMessage}

	_ = expectOneSlice(c0.In, broadcast0, timeout, t)
	_ = expectOneSlice(c1.In, broadcast0, timeout, t)
	expectNoMsg(s.In, timeout, t) //no echo

	s.Out <- reconws.WsMessage{Data: broadcast1, Type: websocket.BinaryMessage}

	_ = expectOneSlice(c0.In, broadcast1, timeout, t)
	_ = expectOneSlice(c1.In, broadcast1, timeout, t)
	expectNoMsg(s.In, timeout, t)  //no echo
	expectNoMsg(c0.In, timeout, t) //only expecting two messages
	expectNoMsg(c1.In, timeout, t) //no third message expected

	time.Sleep(timeout)

	cancel()

	time.Sleep(timeout)

	close(closed)

	wg.Wait()

}

func expectNoMsg(channel chan reconws.WsMessage, timeout time.Duration, t *testing.T) {

	select {
	case <-time.After(timeout):
		return //we are expecting to timeout, this is good
	case msg, ok := <-channel:
		if ok {
			t.Errorf("Receieved unexpected message %v", msg)
		} else {
			//just a channel problem, not an unexpected message
		}
	}
}

func expectOneSlice(channel chan reconws.WsMessage, expected []byte, timeout time.Duration, t *testing.T) []byte {

	var receivedSlice []byte

	select {
	case <-time.After(timeout):
		t.Errorf("timeout receiving message (expected %s)", expected)
	case msg, ok := <-channel:
		if ok {
			receivedSlice = msg.Data
			if bytes.Compare(receivedSlice, expected) != 0 {
				t.Errorf("Messages don't match: Want: %s\nGot : %s\n", expected, receivedSlice)
			}
		} else {
			t.Error("Channel problem")
		}
	}
	return receivedSlice
}
