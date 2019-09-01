package cmd

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message, host *url.URL) {

	defer func() {
		wg.Done()
		log.WithFields(log.Fields{
			"func": "HandleConnections",
			"verb": "closed",
		}).Trace("HandleConnections closed")
	}()

	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

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

		var HTTPUpgrader ws.HTTPUpgrader

		// chrome needs this to connect
		HTTPUpgrader.Protocol = func(str string) bool { return true }

		conn, _, _, err := HTTPUpgrader.Upgrade(r, w)

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

				msg, op, err := wsutil.ReadClientData(conn)

				if err == nil {

					// pass on the message ... (assume handleMessages will do stats)
					messagesFromMe <- message{sender: client, op: op, data: msg}

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
				conn.Close()
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

					err = wsutil.WriteServerMessage(conn, msg.op, msg.data)

					if err != nil {

						log.WithFields(log.Fields{
							"error":  err,
							"func":   "HandlerFunc.Write",
							"name":   name,
							"r":      r,
							"topic":  topic,
							"msg.op": msg.op,
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

	}) //end of fn (HandlerFunc) definition

	addr := strings.Join([]string{host.Hostname(), ":", host.Port()}, "")

	log.WithFields(log.Fields{
		"func": "HandleConnections",
		"verb": "starting",
		"addr": addr,
	}).Info("Starting http.ListenAndServe")

	//TODO recast this so that it can be cancelled
	err := http.ListenAndServe(addr, fn)

	if err != nil {

		log.WithFields(log.Fields{
			"error": err,
			"func":  "HandleConnections",
			"addr":  addr,
		}).Fatal("Error with http.ListenAndServe")
	}
}
