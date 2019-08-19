package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message) {
	defer wg.Done()

	var buf = make([]byte, bufferSize)
	var n int

	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			fmt.Println(err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "HandleConnection: the sky is falling")

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		//subscribe this new client
		var messagesForMe chan message
		var name = uuid.New().String()
		var topic = r.URL.Path

		client := clientDetails{name: name, topic: topic, messagesChan: messagesForMe}

		clientActionsChan <- clientAction{action: clientAdd, client: client}

		defer func() {
			clientActionsChan <- clientAction{action: clientDelete, client: client}
		}()

		//typ, reader, err := c.Reader(ctx)
		//n, err = reader.Read(buf)
		//fmt.Printf("%v %v", typ, n)

		for {
			select {
			default:

				typ, reader, err := c.Reader(ctx)

				if err != nil {
					fmt.Println("HandleConnections: io.Reader", err)
				}

				if typ != websocket.MessageBinary {
					fmt.Println("Not binary")
				}

				n, err = reader.Read(buf)

				if err != nil {
					if err != io.EOF {
						fmt.Println("Read:", err)
					}
				}
				fmt.Printf("%v %v %v\n", typ, n, buf[:n])
				messagesFromMe <- message{sender: client, typ: typ, data: buf[:n]}

			case <-closed:
				c.Close(websocket.StatusNormalClosure, "")
				return
			}
		}

		go func() {
			for {
				select {
				case msg := <-messagesForMe:
					w, err := c.Writer(ctx, msg.typ)

					if err != nil {
						fmt.Println("HandleConnections: io.Writer", err)
					}

					n, err = w.Write(msg.data)

					if n != len(buf) {
						fmt.Println("HandleConnetions: Mismatch write lengths, overflow?")

					}

					if err != nil {
						if err != io.EOF {
							fmt.Println("Write:", err)
						}
					}

					err = w.Close() // do every write to flush frame
					if err != nil {
						fmt.Println("Closing Write failed:", err)
					}
				case <-closed:
					return
				}
			}
		}()

	}) //end of fun definition

	addr := strings.Join([]string{host.Hostname(), ":", host.Port(), "/"}, "")
	log.Printf("Starting listener on %s\n", addr)
	err := http.ListenAndServe("localhost:8097", fn)
	log.Fatal(err)

}
