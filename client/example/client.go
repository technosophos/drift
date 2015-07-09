package main

import (
	"fmt"
	"io"
	//"io/ioutil"
	"os"
	//"net"
	"bytes"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/bradfitz/http2"
	"github.com/bradfitz/http2/hpack"
	"github.com/technosophos/drift/client"
	"github.com/technosophos/drift/transport"
)

func main() {

	cmd := os.Args[1]

	/*
		//t := &http2.Transport{InsecureTLSDial: true}
		t := &transport.Transport{InsecureTLSDial: true}

		req, _ := http.NewRequest("GET", "https://localhost:5500/", nil)

		//res, err := t.RoundTrip(req)
		res, stream, err := t.Listen(req)
		if err != nil {
			fmt.Printf("Failed: %s\n", err)
		}
		data, _ := ioutil.ReadAll(res.Body)
		fmt.Printf("Response: %s %d, %q\n", res.Proto, res.StatusCode, data)

		fmt.Println("Waiting for the next message.")
		moredata := <-stream
		fmt.Printf("Final data: %s", moredata)

		// Next, hit the ticker
		getTicker()
	*/
	switch cmd {
	case "publish":
		//publish()
		p := client.NewPublisher("https://localhost:5500")
		p.Publish("example", []byte("Hello World"))
		time.Sleep(200 * time.Millisecond)
		p.Publish("example", []byte("Hello again"))
	case "subscribe":
		//subscribe()
		s := client.NewSubscriber("https://localhost:5500")
		s.History = client.History{Len: 5}
		stream, err := s.Subscribe("example")
		if err != nil {
			fmt.Printf("Failed subscription: %s", err)
			return
		}
		for msg := range stream {
			fmt.Printf("Received: %s\n", msg)
		}
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
	}
}

func publish() {

	/* HTTP2 does not currently send the body! So we have to go to HTTP1
	t := &transport.Transport{InsecureTLSDial: true}
	*/
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	var body bytes.Buffer

	body.Write([]byte("test"))

	req, _ := http.NewRequest("POST", "https://localhost:5500/v1/t/TEST", &body)

	res, err := t.RoundTrip(req)
	if err != nil {
		fmt.Printf("Error during round trip: %s", err)
	}

	fmt.Printf("Status: %s", res.Status)

}

func subscribe() {
	t := &transport.Transport{InsecureTLSDial: true}

	req, _ := http.NewRequest("GET", "https://localhost:5500/v1/t/TEST", nil)

	//res, err := t.RoundTrip(req)
	res, stream, err := t.Listen(req)
	if err != nil {
		fmt.Printf("Failed: %s\n", err)
	}
	fmt.Printf("Headers: %s\n", res.Status)
	//data, _ := ioutil.ReadAll(res.Body)
	//fmt.Printf("Response: %s %d, %q\n", res.Proto, res.StatusCode, data)

	fmt.Println("Waiting for the next message.")
	for data := range stream {
		fmt.Printf("\nReceived: %s\n", data)
	}
}

func getTicker() *http.Response {
	t := &transport.Transport{InsecureTLSDial: true}

	req, _ := http.NewRequest("GET", "https://localhost:5500/tick", nil)

	//res, err := t.RoundTrip(req)
	res, stream, err := t.Listen(req)
	if err != nil {
		fmt.Printf("Failed: %s\n", err)
	}

	fmt.Printf("Status: %s\n", res.Status)
	fmt.Printf("Body: %s\n", res.Body)

	for data := range stream {
		fmt.Printf("\nReceived: %s\n", data)
	}

	//io.Copy(os.Stdout, res.Body)
	return res
}

func custom() {
	dest := "localhost"
	port := ":5500"
	// Create a new client.

	tlscfg := &tls.Config{
		ServerName:         dest,
		NextProtos:         []string{http2.NextProtoTLS},
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", dest+port, tlscfg)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	conn.Handshake()
	state := conn.ConnectionState()
	fmt.Printf("Protocol is : %q\n", state.NegotiatedProtocol)

	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		fmt.Printf("Preface failed: %s", err)
		return
	}

	var hbuf bytes.Buffer
	framer := http2.NewFramer(conn, conn)

	enc := hpack.NewEncoder(&hbuf)
	writeHeader(enc, ":authority", "localhost")
	writeHeader(enc, ":method", "GET")
	writeHeader(enc, ":path", "/ping")
	writeHeader(enc, ":scheme", "https")
	writeHeader(enc, "Accept", "*/*")

	if len(hbuf.Bytes()) > 16<<10 {
		fmt.Printf("Need CONTINUATION\n")
	}

	headers := http2.HeadersFrameParam{
		StreamID:      1,
		EndStream:     true,
		EndHeaders:    true,
		BlockFragment: hbuf.Bytes(),
	}

	fmt.Printf("All the stuff: %q\n", headers.BlockFragment)

	go listen(framer)

	framer.WriteSettings()
	framer.WriteWindowUpdate(0, 1<<30)
	framer.WriteSettingsAck()

	time.Sleep(time.Second * 2)
	framer.WriteHeaders(headers)
	time.Sleep(time.Second * 2)

	/* A ping HTTP request
	var payload [8]byte
	copy(payload[:], "_c0ffee_")
	framer.WritePing(false, payload)
	rawpong, err := framer.ReadFrame()
	if err != nil {
		panic(err)
	}

	pong, ok := rawpong.(*http2.PingFrame)
	if !ok {
		fmt.Printf("Instead of a Ping, I got this: %v\n", pong)
		return
	}

	fmt.Printf("Pong: %q\n", pong.Data)
	*/

}

func listen(framer *http2.Framer) {
	for {
		response, err := framer.ReadFrame()
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Printf("Error: Got %q\n", err)
		}
		switch t := response.(type) {
		case *http2.SettingsFrame:
			t.ForeachSetting(func(s http2.Setting) error {
				fmt.Printf("Setting: %q\n", s)
				return nil
			})
		case *http2.GoAwayFrame:
			fmt.Printf("Go Away code = %q, stream ID = %d\n", t.ErrCode, t.StreamID)
		}
		//data := response.(*http2.DataFrame)
		fmt.Printf("Got %q\n", response)
	}
}

func writeHeader(enc *hpack.Encoder, name, value string) {
	enc.WriteField(hpack.HeaderField{Name: name, Value: value})
}
