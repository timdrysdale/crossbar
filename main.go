// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	
)

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

func main() {

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	cleanupDone := make(chan struct{})
	
	flag.Parse()
	//hub := newHub()
	go h.run()
	go timer()

	go func() {
		fmt.Println("\nMain waiting on signalChan...\n")
		<-signalChan
		fmt.Println("\nMain Received an interrupt, stopping services...\n")
		//cleanup(services, c)
		close(cleanupDone)
	}()
	
        go func(){
	r := mux.NewRouter()
	r.HandleFunc("/", serveHome)
	r.HandleFunc("/ws/{room}/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r)  //was hub, w, r
	})
	http.Handle("/", r)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	}()

	<-cleanupDone
	
}
