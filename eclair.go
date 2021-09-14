package eclair

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

type Client struct {
	Host     string
	Password string
}

func (c Client) baseURL() string {
	if strings.HasPrefix(c.Host, "http") {
		return c.Host
	}
	return "http://" + c.Host
}

func (c Client) authorizationHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+c.Password))
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Params map[string]interface{}

func (c *Client) Call(method string, data map[string]interface{}) (gjson.Result, error) {
	r, err := http.NewRequest("POST", c.baseURL()+"/"+method, nil)
	if err != nil {
		return gjson.Result{},
			fmt.Errorf("error creating http request to %s: %w", c.Host, err)
	}

	r.Header.Set("Accept", "application/json")
	r.Header.Set("Authorization", c.authorizationHeader())

	if data != nil {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for k, v := range data {
			fw, err := writer.CreateFormField(k)
			if err != nil {
				return gjson.Result{},
					fmt.Errorf("error creating form field %s: %w", k, err)
			}
			_, err = io.Copy(fw, strings.NewReader(fmt.Sprintf("%v", v)))
			if err != nil {
				return gjson.Result{},
					fmt.Errorf("error adding field value %s=%v: %w", k, v, err)
			}
		}

		if err := writer.Close(); err != nil {
			return gjson.Result{}, fmt.Errorf("error closing form-data writer: %w", err)
		}

		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Body = ioutil.NopCloser(bytes.NewReader(body.Bytes()))
	}

	w, err := http.DefaultClient.Do(r)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("call to %s errored: %w", c.Host, err)
	}
	defer w.Body.Close()

	b, err := ioutil.ReadAll(w.Body)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if w.StatusCode >= 300 {
		var errorResponse ErrorResponse
		if err := json.Unmarshal(b, &errorResponse); err != nil {
			text := string(b)
			if len(text) > 200 {
				text = text[:200]
			}
			return gjson.Result{},
				fmt.Errorf("failed to decode json error response '%s': %w", text, err)
		}

		return gjson.Result{}, fmt.Errorf("eclair said: %s", errorResponse.Error)
	}

	var response interface{}
	if err := json.Unmarshal(b, &response); err != nil {
		text := string(b)
		if len(text) > 200 {
			text = text[:200]
		}
		return gjson.Result{},
			fmt.Errorf("failed to decode json good response '%s': %w", text, err)
	}

	// now that we know the response is good json, we use gjson
	return gjson.ParseBytes(b), nil
}

func (c *Client) Websocket() (<-chan gjson.Result, error) {
	url := strings.Replace(c.baseURL(), "http", "ws", 1) + "/ws"

	messages := make(chan gjson.Result)
	go func() {
		defer close(messages)
	retry:
		conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
			"Authorization": {c.authorizationHeader()},
		})
		if err != nil {
			log.Println("failed to open websocket connection: " + err.Error())
		}
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "connection reset by peer") {
					log.Println("lost websocket to eclair, reconnecing in 5 seconds")
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
