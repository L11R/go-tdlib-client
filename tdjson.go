package main

//#cgo LDFLAGS: -ltdjson
//#include <stdlib.h>
//#include <td/telegram/td_json_client.h>
//#include <td/telegram/td_log.h>
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
	"unsafe"
)

type Update = map[string]interface{}

type Client struct {
	Client  unsafe.Pointer
	Updates chan Update
	Waiters sync.Map
}

func NewClient() *Client {
	client := Client{Client: C.td_json_client_create()}
	client.Updates = make(chan map[string]interface{}, 100)

	go func() {
		for {
			event := client.Receive(10)

			var update Update
			json.Unmarshal([]byte(event), &update)

			if extra, hasExtra := update["@extra"].(string); hasExtra {
				if waiter, found := client.Waiters.Load(extra); found {
					waiter.(chan Update) <- update
					close(waiter.(chan Update))
				}
			} else {
				if _, ok := update["@type"]; ok {
					client.Updates <- update
				} else {
					fmt.Println("update without @type field")
				}
			}
		}
	}()

	return &client
}

func (c *Client) Destroy() {
	C.td_json_client_destroy(c.Client)
}

func (c *Client) Send(jsonQuery string) {
	query := C.CString(jsonQuery)
	defer C.free(unsafe.Pointer(query))

	C.td_json_client_send(c.Client, query)
}

func (c *Client) Receive(timeout float64) string {
	result := C.td_json_client_receive(c.Client, C.double(timeout))
	return C.GoString(result)
}

func (c *Client) Execute(jsonQuery string) string {
	query := C.CString(jsonQuery)
	defer C.free(unsafe.Pointer(query))

	result := C.td_json_client_execute(c.Client, query)
	return C.GoString(result)
}

func SetFilePath(path string) {
	query := C.CString(path)
	defer C.free(unsafe.Pointer(query))

	C.td_set_log_file_path(query)
}

func SetLogVerbosityLevel(level int) {
	C.td_set_log_verbosity_level(C.int(level))
}

func (c *Client) SendAndCatch(jsonQuery string) (map[string]interface{}, error) {
	// unmarshal JSON into map, we don't have @extra field, if user don't set it
	var jsonWithoutExtra map[string]interface{}
	json.Unmarshal([]byte(jsonQuery), &jsonWithoutExtra)

	// letters for generating random string
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// generate random string for @extra field
	b := make([]byte, 32)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	randomString := string(b)

	// marshal new json with @extra field
	jsonWithoutExtra["@extra"] = randomString
	jsonWithExtra, _ := json.Marshal(&jsonWithoutExtra)

	waiter := make(chan Update, 1)
	c.Waiters.Store(randomString, waiter)

	// send it through already implemented method
	c.Send(string(jsonWithExtra))

	// wait response or timeout
	select {
	case response := <-waiter:
		fmt.Println("catched:", response)
		return response, nil
	case <-time.After(10 * time.Second):
		fmt.Println("timeout")
		c.Waiters.Delete(randomString)
		return Update{}, errors.New("timeout")
	}
}
