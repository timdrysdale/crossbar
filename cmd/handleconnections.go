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
		messagesForMe := make(chan message, 2)
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
		readerReturnsChan := make(chan readerReturns)
		readerFinishedChan := make(chan int)
		readerFinishedChan <- 0 //start the cycle off with a wait for a read

		go func() {
			for {
				select {
				case <-readerFinishedChan:
					typ, reader, err := c.Reader(ctx)
					readerReturnsChan <- readerReturns{typ, reader, err}
				default:

				}
			}
		}()

		for {
			select {

			case readerDetails := <-readerReturnsChan:

				if readerDetails.err != nil {
					//fmt.Println("HandleConnections: io.Reader", err)
					return
				}

				//if typ != websocket.MessageBinary {
				//	fmt.Println("Not binary")
				//}

				n, err = readerDetails.reader.Read(buf)

				if err != nil {
					if err != io.EOF {
						fmt.Println("Read:", err)
					}
				}
				//fmt.Printf("%v %v %v\n", typ, n, buf[:n])
				messagesFromMe <- message{sender: client, typ: readerDetails.typ, data: buf[:n]}
				readerFinishedChan <- 0

			case msg := <-messagesForMe:
				w, err := c.Writer(ctx, msg.typ)

				if err != nil {
					fmt.Println("HandleConnections: io.Writer", err)
				}

				n, err = w.Write(msg.data)

				if n != len(msg.data) {
					fmt.Println("HandleConnections: Mismatch write lengths, overflow?")

				}

				if err != nil {
					if err != io.EOF {
						fmt.Println("Write:", err)
					}
				} else {
					fmt.Printf("ws send to %v %v\n", name, buf[:n])
				}

				err = w.Close() // do every write to flush frame
				if err != nil {
					fmt.Println("Closing Write failed:", err)
				}
			case <-closed:
				c.Close(websocket.StatusNormalClosure, "")
				return
			}
		}

	}) //end of fun definition

	addr := strings.Join([]string{host.Hostname(), ":", host.Port(), "/"}, "")
	log.Printf("Starting listener on %s\n", addr)
	err := http.ListenAndServe("localhost:8097", fn)
	log.Fatal(err)

}
