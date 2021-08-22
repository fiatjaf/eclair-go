package eclair

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type Client struct {
	Host     string
	Password string
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Params map[string]interface{}

func (c *Client) Call(method string, data map[string]interface{}) (gjson.Result, error) {
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

	r, err := http.NewRequest("POST", c.Host, bytes.NewReader(body.Bytes()))
	if err != nil {
		return gjson.Result{},
			fmt.Errorf("error creating http request to %s: %w", c.Host, err)
	}

	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Authorization",
		base64.StdEncoding.EncodeToString([]byte(":"+c.Password)))

	w, err := (&http.Client{Timeout: time.Second * 10}).Do(r)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("call to %s errored: %w", c.Host, err)
	}

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
