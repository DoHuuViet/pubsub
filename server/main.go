package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

func main() {
	startServer()
}

func startServer() {

	requestsCh := make(chan *http.Request)
	channel := NewChannel()

	http.HandleFunc("/", proxyHandler(requestsCh, channel))
	http.HandleFunc("/ws", serveWs(requestsCh, channel))

	if err := http.ListenAndServe(fmt.Sprintf(":%d", 9000), nil); err != nil {
		log.Fatal(err)
	}
}

func proxyHandler(requests chan *http.Request, channel *Channel) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()
		}

		inletsID := uuid.Formatter(uuid.NewV4(), uuid.FormatHex)
		log.Printf("router-start -> [%s]\n", inletsID)

		wg := sync.WaitGroup{}

		sub := channel.Subscribe(inletsID)

		r.Header.Set("inlets-id", inletsID)

		wg.Add(2)
		var match bool
		var res *http.Response

		// Add receiver first
		go func(sub *Subscription) {
		recv:
			select {
			case recvRes := <-sub.Data:
				match = recvRes.Header.Get("inlets-id") == inletsID
				log.Printf("router-sub-recv: %s=%v, %v", inletsID, sub.Data, match)

				if !match {
					goto recv
				}
				res = recvRes
				break
			case <-time.After(time.Second * 4):
				log.Println("Timeout after 4s")
				goto subdone
			}

		subdone:

			wg.Done()
		}(sub)

		// sender
		go func() {
			log.Printf("router-send [%s]\n", inletsID)

			requests <- r
			wg.Done()
		}()

		wg.Wait()

		defer func() {
			channel.Unsubscribe(inletsID)
			log.Printf("router-remove [%s]\n", inletsID)
		}()

		if res == nil {
			w.WriteHeader(http.StatusGatewayTimeout)
			w.Write([]byte("Timeout\n"))
			return
		}

		w.WriteHeader(res.StatusCode)
		w.Write([]byte("Done\n"))
		log.Printf("router-sent [%s]\n", inletsID)

	}
}

func serveWs(requests chan *http.Request, channel *Channel) func(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log.Println(err)
			}
			return
		}

		done := make(chan bool)

		// receive requests from router handler and dispatch to websocket
		go func() {
			defer close(done)

			for request := range requests {

				reqBuf := new(bytes.Buffer)
				writeErr := request.Write(reqBuf)
				if writeErr != nil {
					log.Println(writeErr)
				}

				log.Println("Sending request.")
				ws.WriteMessage(websocket.BinaryMessage, reqBuf.Bytes())
			}
		}()

		// receive responses from websocket and dispatch to router handler
		go func() {
			defer close(done)

			for {
				msgType, msg, err := ws.ReadMessage()

				if err != nil {
					log.Println(err)
				}

				if msgType == websocket.BinaryMessage {

					resBuf := new(bytes.Buffer)
					resBuf.Write(msg)

					resReader := bufio.NewReader(resBuf)
					res, reqErr := http.ReadResponse(resReader, nil)

					if reqErr != nil {
						log.Println(reqErr)
					}

					fmt.Printf("ws-recv [%s]\n", res.Header.Get("inlets-id"))
					subs := 0
					sent := 0
					// Dispatch via subscriptions
					subsList := channel.SubscriptionList()
					totals := len(subsList)
					for i, subKey := range subsList {
						if subKey == res.Header.Get("inlets-id") {
							fmt.Printf("ws-ch-send %d/%d\n", (i + 1), totals)

							channel.Send(subKey, res)
							sent = sent + 1
						}
						subs++
					}

					log.Printf("%d subs processed, subs sent: %d\n", subs, sent)

				}
			}
		}()

		<-done
	}
}
