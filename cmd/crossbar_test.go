package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/timdrysdale/reconws"
)

func MakeTestToken(audience string, lifetime int64, secret string) (string, error) {

	now := time.Now().Unix()
	later := now + lifetime
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud": audience,
		"iat": now,
		"nbf": now,
		"exp": later,
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(secret))

	return tokenString, err

}

func TestAuth(t *testing.T) {
	//log.SetLevel(log.TraceLevel)
	suppressLog()
	defer displayLog()

	//Todo - add support for httptest https://stackoverflow.com/questions/40786526/resetting-http-handlers-in-golang-for-unit-testing
	http.DefaultServeMux = new(http.ServeMux)

	// setup crossbar on local (free) port

	closed := make(chan struct{})
	var wg sync.WaitGroup

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}

	addr := ":" + strconv.Itoa(port)
	route := "ws://127.0.0.1" + addr
	secret := "asldjflkasjdflkj13094809asdfhkj13"
	config := Config{
		Addr:     addr,
		Audience: route,
		Secret:   secret,
	}

	wg.Add(1)

	go crossbar(config, closed, &wg)

	time.Sleep(10 * time.Millisecond)

	// set up test server and two clients

	ctx, cancel := context.WithCancel(context.Background())

	clientEndPoint := "/out/some/location"
	serverEndPoint := "/in/some/location"
	uc := route + clientEndPoint
	us := route + serverEndPoint

	var lifetime int64 = 999999
	ctoken, err := MakeTestToken(uc, lifetime, secret)

	assert.NoError(t, err)
	stoken, err := MakeTestToken(us, lifetime, secret)

	assert.NoError(t, err)

	c0 := reconws.New()
	c1 := reconws.New()
	s := reconws.New()

	go c0.Reconnect(ctx, uc)
	go c1.Reconnect(ctx, uc)
	go s.Reconnect(ctx, us)
	fmt.Printf("server connection at %s\n", us)

	timeout := 50 * time.Millisecond

	time.Sleep(timeout)

	// do authorisation
	mtype := websocket.TextMessage

	c0.Out <- reconws.WsMessage{Data: []byte(ctoken), Type: mtype}
	c1.Out <- reconws.WsMessage{Data: []byte(ctoken), Type: mtype}
	s.Out <- reconws.WsMessage{Data: []byte(stoken), Type: mtype}

	expectedClientReply, err := json.Marshal(AuthMessage{
		Topic:      clientEndPoint,
		Token:      ctoken,
		Authorised: true,
		Reason:     "ok",
	})

	assert.NoError(t, err)

	_ = expectOneSlice(c0.In, expectedClientReply, timeout, t)
	_ = expectOneSlice(c1.In, expectedClientReply, timeout, t)

	expectedServerReply, err := json.Marshal(AuthMessage{
		Topic:      serverEndPoint,
		Token:      stoken,
		Authorised: true,
		Reason:     "ok",
	})

	_ = expectOneSlice(s.In, expectedServerReply, timeout, t)

	payload0 := []byte("Hello from client0")
	payload1 := []byte("Hello from client1")

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

func TestCrossbarUniDirectionalMessaging(t *testing.T) {

	suppressLog()
	defer displayLog()

	//Todo - add support for httptest https://stackoverflow.com/questions/40786526/resetting-http-handlers-in-golang-for-unit-testing
	http.DefaultServeMux = new(http.ServeMux)

	// setup crossbar on local (free) port

	closed := make(chan struct{})
	var wg sync.WaitGroup

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}

	addr := ":" + strconv.Itoa(port)
	config := Config{
		Addr: addr,
	}
	wg.Add(1)
	go crossbar(config, closed, &wg)

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

func TestCrossbarBidirectionalMessaging(t *testing.T) {

	suppressLog()
	defer displayLog()

	//TODO - add support for httptest https://stackoverflow.com/questions/40786526/resetting-http-handlers-in-golang-for-unit-testing
	http.DefaultServeMux = new(http.ServeMux)

	// setup crossbar on local (free) port

	closed := make(chan struct{})
	var wg sync.WaitGroup

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}

	addr := ":" + strconv.Itoa(port)

	config := Config{
		Addr: addr,
	}

	wg.Add(1)
	go crossbar(config, closed, &wg)

	time.Sleep(10 * time.Millisecond)

	// set up test server and two clients

	ctx, cancel := context.WithCancel(context.Background())

	uc := "ws://127.0.0.1" + addr + "/bi/some/location"
	us := "ws://127.0.0.1" + addr + "/bi/some/location"

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

	//this message goes to s and c1
	c0.Out <- reconws.WsMessage{Data: payload0, Type: mtype}
	_ = expectOneSlice(s.In, payload0, timeout, t)
	_ = expectOneSlice(c1.In, payload0, timeout, t)
	expectNoMsg(c0.In, timeout, t) //should not see message from self

	//this message goes to s and c0
	c1.Out <- reconws.WsMessage{Data: payload1, Type: mtype}
	_ = expectOneSlice(s.In, payload1, timeout, t)
	_ = expectOneSlice(c0.In, payload1, timeout, t)
	expectNoMsg(c1.In, timeout, t) //should not see message from self

	// the server should get each message only once
	expectNoMsg(s.In, timeout, t)

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
		if ok && len(expected) > 0 {
			receivedSlice = msg.Data
			if bytes.Compare(receivedSlice, expected) != 0 {
				t.Errorf("Messages don't match: Want: %s\nGot : %s\n", expected, receivedSlice)
			}
		} else if !ok {
			t.Error("Channel problem")
		} else { //for the case we didn't know in advance the reply type ....
			// use this only for debugging tests
			receivedSlice = msg.Data
		}
	}
	return receivedSlice
}
