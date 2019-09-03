package cmd

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

/*

func HandleMessages(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, messagesChan <-chan message) {

	defer func() {
		log.WithFields(log.Fields{
			"func": "HandleMessages",
			"verb": "closed",
		}).Trace("HandleMessages closed")

		wg.Done()
	}()

	wg.Add(1)

	statsChan := make(chan messageStats, 5) //avoid blocking

	go collectStats(closed, wg, statsChan)

	for {
		select {
		case <-closed:
			return
		case msg := <-messagesChan:
			distributeMessage(topics, msg, statsChan)
		}
	}
}

func distributeMessage(topics *topicDirectory, msg message, statsChan chan messageStats) {

	topics.Lock()
	distributionList := topics.directory[msg.sender.topic]
	topics.Unlock()

	//unsubscribing does not close a channel so we are safe to write
	//to a client that unsubscribes between us getting the topic list
	//and sending the message.
	//a channel persists so long as there is a reference to it
	//channels can be left open - unreachable channels are GC

	stats := messageStats{topic: msg.sender.topic, rx: make([]string, 0), size: len(msg.data)}

	for _, destination := range distributionList {

		//don't send to sender
		if destination.name != msg.sender.name {

			//collect name for stats purposes
			stats.rx = append(stats.rx, destination.name)

			//buffered channels, so non-blocking write
			destination.messagesChan <- msg
			select {
			case destination.messagesChan <- msg:
			default:
				close(destination.messagesChan)
			}
		}
	}

	statsChan <- stats
}

// collectStats reduces log pollution by collating message stats over
// 60 second periods

func collectStats(closed <-chan struct{}, wg *sync.WaitGroup, incoming <-chan messageStats) {
	defer func() {
		log.WithFields(log.Fields{
			"func": "HandleMessages.collectStats",
			"verb": "closed",
		}).Trace("HandleMessage.collectStats closed")
		wg.Done()
	}()

	stats := summaryStats{}
	stats.topic = make(map[string]topicStats)

	period := 10 * time.Second

	report := time.NewTicker(period)

	for {

		select {
		case <-closed:
			return
		case <-report.C:
			for name, topic := range stats.topic {
				log.WithFields(log.Fields{
					"period":           period,
					"topic":            name,
					"msg.count":        topic.size.Count(),
					"msg.max":          topic.size.Max(),
					"msg.min":          topic.size.Min(),
					"msg.avg":          math.Round(topic.size.Mean()),
					"audience.max":     topic.audience.Max(),
					"audience.min":     topic.audience.Min(),
					"audience.avg":     math.Round(topic.audience.Mean()*100) / 100, //2dp
					"audience.members": topic.rx,
				}).Info("Topic stats")
			}
			resetStats(&stats)

		case msg := <-incoming:

			if _, ok := stats.topic[msg.topic]; !ok {
				stats.topic[msg.topic] = topicStats{audience: welford.New(), size: welford.New(), rx: make(map[string]int)}
			}

			for _, rxer := range msg.rx {
				if _, ok := stats.topic[msg.topic].rx[rxer]; ok {
					stats.topic[msg.topic].rx[rxer] += 1
				} else {
					stats.topic[msg.topic].rx[rxer] = 1
				}
			}

			stats.topic[msg.topic].audience.Add(float64(len(msg.rx)))
			stats.topic[msg.topic].size.Add(float64(msg.size))
		}
	}
}

func resetStats(stats *summaryStats) {

	for topic, _ := range stats.topic {

		if stats.topic[topic].size.Count() == 0 {
			delete(stats.topic, topic)
		} else {
			stats.topic[topic].audience.Reset()
			stats.topic[topic].size.Reset()
			for k := range stats.topic[topic].rx {
				delete(stats.topic[topic].rx, k)
			}
		}
	}
}
*/
