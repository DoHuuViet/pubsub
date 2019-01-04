package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

func main() {

	u := url.URL{Scheme: "ws", Host: "localhost:9000", Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err != nil {
		panic(err)
	}

	fmt.Println(ws.LocalAddr())

	defer ws.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)

		for {
			messageType, message, err := ws.ReadMessage()
			if err != nil {
				log.Println(err)
			}

			if messageType == websocket.BinaryMessage {

				reqBuf := bytes.NewBuffer(message)
				bufReader := bufio.NewReader(reqBuf)
				req, reqErr := http.ReadRequest(bufReader)

				if reqErr != nil {
					log.Println(reqErr)
				}

				log.Printf(">Recv %s\n", req.Header.Get("inlets-id"))

				res := http.Response{
					Header: http.Header{
						"inlets-id": []string{req.Header.Get("inlets-id")},
					},
					StatusCode: http.StatusOK,
					Status:     "OK",
				}

				resBuf := new(bytes.Buffer)
				res.Write(resBuf)
				log.Printf("<Send %s\n", req.Header.Get("inlets-id"))

				ws.WriteMessage(websocket.BinaryMessage, resBuf.Bytes())

			}
		}
	}()

	<-done

}
