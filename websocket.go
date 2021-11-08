package eclair

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

func (c *Client) Websocket() (<-chan gjson.Result, error) {
	url := strings.Replace(c.baseURL(), "http", "ws", 1) + "/ws"

	messages := make(chan gjson.Result)
	go func() {
		ticker := time.NewTicker(time.Second * 29)
		defer ticker.Stop()
		defer close(messages)

		var err error
		var conn *websocket.Conn

		go func() {
			for {
				<-ticker.C
				conn.WriteMessage(websocket.PingMessage, nil)
			}
		}()

	retry:
		conn, _, err = websocket.DefaultDialer.Dial(url, http.Header{
			"Authorization": {c.authorizationHeader()},
		})
		if err != nil {
			log.Println("failed to open websocket connection: " + err.Error())
		}
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "connection reset by peer") {
					log.Println("lost websocket to eclair, reconnecting in 5 seconds")
					time.Sleep(time.Second * 5)
					goto retry
				}

				log.Println("eclair ws read error: ", err.Error())
				return
			}
			messages <- gjson.ParseBytes(message)
		}
	}()

	return messages, nil
}
