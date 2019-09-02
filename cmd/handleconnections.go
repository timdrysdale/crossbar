package cmd

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	log "github.com/sirupsen/logrus"
)

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
	//this disnae work for gorilla strinliteral error ... upgrader.Subprotocols = make([]string{"nil"})

	//consider increasing buffer size (check traffic and performance first ...)

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

			case msg := <-messagesForMe:

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
