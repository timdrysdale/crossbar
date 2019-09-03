package cmd

import (
	"flag"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	log "github.com/sirupsen/logrus"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer (10MB)
	// Typical key frame at 640x480 is 60 * 188B ~= 11kB
	maxMessageSize = 1024 * 1024 * 10
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// 4096 Bytes is the approx average message size
// this number does not limit message size
// So for key frames we just make a few more syscalls
// null subprotocol required by Chrome
// TODO restrict CheckOrigin
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	Subprotocols:    []string{"null"},
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Client is a middleperson between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// string representing the path the client connected to
	topic string
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("error: %v", err)
			}
			break
		}

		c.hub.broadcast <- message
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chunks to the current websocket message, without delimiter.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

var addr = flag.String("addr", ":8080", "http service address")

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

/*func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}*/

func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message, host *url.URL) {
	hub := newHub()
	go hub.run()
	//	http.HandleFunc("/", serveHome)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	wg.Done()
}

/*

func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message, host *url.URL) {

	defer func() {
		log.WithFields(log.Fields{
			"func": "HandleConnections",
			"verb": "closed",
		}).Trace("HandleConnections closed")
		wg.Done()
	}()

	addr := strings.Join([]string{host.Hostname(), ":", host.Port()}, "")

	log.WithFields(log.Fields{
		"func": "HandleConnections",
		"verb": "starting",
		"addr": addr,
	}).Info("Starting http.ListenAndServe")

	// wrap in the extra arguments we need
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { relay(w, r, clientActionsChan, messagesFromMe, closed) })

	err := http.ListenAndServe(addr, nil)

	if err != nil {

		log.WithFields(log.Fields{
			"error": err,
			"func":  "HandleConnections",
			"addr":  addr,
		}).Fatal("Error with http.ListenAndServe")
	}
}

func relay(w http.ResponseWriter, r *http.Request, clientActionsChan chan clientAction, messagesFromMe chan message, closed <-chan struct{}) {

	name := uuid.New().String()
	topic := r.URL.Path

	log.WithFields(log.Fields{
		"func":  "HandlerFunc",
		"name":  name,
		"r":     r,
		"topic": topic,
		"verb":  "started",
	}).Trace("HandlerFunc started")

	defer func() {
		log.WithFields(log.Fields{
			"func":  "HandlerFunc",
			"name":  name,
			"r":     r,
			"topic": topic,
			"verb":  "stopped",
		}).Trace("HandlerFunc stopped")
	}()

	var upgrader = websocket.Upgrader{}

	//jsmpeg has a nil subprotocol but chrome needs sec-Websocket-Protocol back ....
	// For gobwas/ws, this worked: HTTPUpgrader.Protocol = func(str string) bool { return true }
	//this disnae work for gorilla strinliteral error ...
	upgrader.Subprotocols = append(upgrader.Subprotocols, "null")

	//consider increasing buffer size (check traffic and performance first ...)

	//TODO - restrict this to practable
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	c, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"func":  "HandlerFunc",
			"name":  name,
			"r":     r,
			"topic": topic,
		}).Fatal("WS upgrade failed")
		return
	}

	defer c.Close()

	//subscribe this new client with a channel to receive messages
	messagesForMe := make(chan message, 3) // TODO make configurable
	client := clientDetails{name: name, topic: topic, messagesChan: messagesForMe}

	log.WithFields(log.Fields{
		"topic":  client.topic,
		"client": client,
		"name":   name,
		"r":      r,
		"verb":   "subscribe",
	}).Debug("Subscribing client to topic") //HandleClients does the info level

	clientActionsChan <- clientAction{action: clientAdd, client: client}

	defer func() {
		clientActionsChan <- clientAction{action: clientDelete, client: client}

		log.WithFields(log.Fields{
			"topic":  client.topic,
			"client": client,
			"name":   name,
			"r":      r,
			"verb":   "unsubscribe",
		}).Debug("Disconnected; unsubscribing client from topic") //HandleClients does the info level
	}()

	var localWG sync.WaitGroup
	localWG.Add(2)

	// read from client
	go func() {

		defer func() {
			log.WithFields(log.Fields{
				"func":  "HandlerFunc.Read",
				"name":  name,
				"r":     r,
				"topic": topic,
				"verb":  "stopped",
			}).Trace("HandlerFunc.Read stopped")
			localWG.Done()
		}()

		for {

			mt, msg, err := c.ReadMessage()

			if err == nil {

				// pass on the message ... (assume handleMessages will do stats)
				messagesFromMe <- message{sender: client, mt: mt, data: msg}

			} else if err == io.EOF {

				// we must be disconnecting, clean up is handled elsewhere
				log.WithFields(log.Fields{
					"func":  "HandlerFunc.Read",
					"name":  name,
					"r":     r,
					"topic": topic,
					"verb":  "EOF",
				}).Trace("HandlerFunc.Read got EOF")

				return

			} else {

				log.WithFields(log.Fields{
					"error": err,
					"func":  "HandlerFunc.Read",
					"name":  name,
					"r":     r,
					"topic": topic,
				}).Warn("Error on WS read")

				return
			}
		}
	}()

	//write to client
	go func() {

		defer func() {
			log.WithFields(log.Fields{
				"func":  "HandlerFunc.Write",
				"name":  name,
				"r":     r,
				"topic": topic,
				"verb":  "stopped",
			}).Trace("HandlerFunc.Write stopped")
			localWG.Done()
		}()

		for {
			select {

			case msg, ok := <-messagesForMe:

				if !ok {
					//our channel has been closed by distributeMessages
					log.WithFields(log.Fields{
						"error":  err,
						"func":   "HandlerFunc.Write",
						"name":   name,
						"r":      r,
						"topic":  topic,
						"msg.mt": msg.mt,
						"len":    len(msg.data),
					}).Warn("Messages channel has been closed...")

					return

				}

				err = c.WriteMessage(msg.mt, msg.data)

				if err != nil {

					log.WithFields(log.Fields{
						"error":  err,
						"func":   "HandlerFunc.Write",
						"name":   name,
						"r":      r,
						"topic":  topic,
						"msg.mt": msg.mt,
						"len":    len(msg.data),
					}).Warn("Error on WS write")

					return
				}

			case <-closed:

				log.WithFields(log.Fields{
					"func":  "HandlerFunc.Write",
					"name":  name,
					"r":     r,
					"topic": topic,
					"verb":  "received close",
				}).Trace("HandlerFunc.Write received close")
				return

			} //select
		} //for
	}() //func

	log.WithFields(log.Fields{
		"func":  "HandlerFunc",
		"name":  name,
		"r":     r,
		"topic": topic,
		"verb":  "waiting",
	}).Trace("HandlerFunc waiting to stop")

	localWG.Wait()

}
*/
