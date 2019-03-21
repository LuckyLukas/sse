// Copyright 2015 Julien Schmidt. All rights reserved.
// Use of this source code is governed by MIT license,
// a copy can be found in the LICENSE file.

package sse

import (
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"
)

type mockResponseWriter struct {
	header  http.Header
	written string
	status  int
}

func (m *mockResponseWriter) Header() (h http.Header) {
	return m.header
}

func (m *mockResponseWriter) Write(p []byte) (n int, err error) {
	m.written += string(p)
	return len(p), nil
}

func (m *mockResponseWriter) WriteString(s string) (n int, err error) {
	m.written += string(s)
	return len(s), nil
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.status = code
}

func NewMockResponseWriter() *mockResponseWriter {
	m := new(mockResponseWriter)
	m.status = 200
	m.header = http.Header{}
	return m
}

type mockResponseWriteFlusher struct {
	*mockResponseWriter
}

func (m mockResponseWriteFlusher) Flush() {}

func NewMockResponseWriteFlusher() mockResponseWriteFlusher {
	return mockResponseWriteFlusher{NewMockResponseWriter()}
}

// all those good old Java times...
type mockResponseWriteFlushCloser struct {
	mockResponseWriteFlusher
	closeNotify chan bool
}

func (m *mockResponseWriteFlushCloser) Close() {
	m.closeNotify <- true
}

func (m *mockResponseWriteFlushCloser) CloseNotify() <-chan bool {
	return m.closeNotify
}

func NewMockResponseWriteFlushCloser() *mockResponseWriteFlushCloser {
	return &mockResponseWriteFlushCloser{
		NewMockResponseWriteFlusher(),
		make(chan bool, 1),
	}
}

func TestNoFlush(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriter()

	time.Sleep(500 * time.Millisecond)

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusNotImplemented {
		t.Fatal("wrong status code:", w.status)
	}
	if w.written != "Flushing not supported\n" {
		t.Fatal("wrong error, got:", w.written)
	}
}

func TestNoClose(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlusher()

	time.Sleep(500 * time.Millisecond)

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusNotImplemented {
		t.Fatal("wrong status code:", w.status)
	}
	if w.written != "Closing not supported\n" {
		t.Fatal("wrong error, got:", w.written)
	}
}

func TestClientConnection(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)
		if len(streamer.clients) != 1 {
			t.Fatal("expected 1 client, has:", len(streamer.clients))
		}
		w.Close()
	}()

	if len(streamer.clients) != 0 {
		t.Fatal("expected 0 clients, has:", len(streamer.clients))
	}
	streamer.ServeHTTP(w, nil)

	time.Sleep(500 * time.Millisecond)
	if len(streamer.clients) != 0 {
		t.Fatal("expected 0 clients, has:", len(streamer.clients))
	}

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}
}

func TestHeader(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	var expected = []struct {
		header string
		value  string
	}{
		{"Cache-Control", "no-cache"},
		{"Connection", "keep-alive"},
		{"Content-Type", "text/event-stream"},
	}
	h := w.Header()

	for _, header := range expected {
		if h.Get(header.header) != header.value {
			t.Errorf(
				"wrong header value for '%s', expected: '%s', got: '%s'",
				header.header,
				header.value,
				h.Get(header.header),
			)
		}
	}
}

func TestSendByte(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	var expected string

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)

		streamer.SendBytes("", "empty", nil)
		expected += "event:empty\ndata:\n\n"

		streamer.SendBytes("", "error", []byte("gnah"))
		expected += "event:error\ndata:gnah\n\n"

		streamer.SendBytes("", "", []byte("\nline\nbreak\n\n"))
		expected += "data:\ndata:line\ndata:break\ndata:\ndata:\n\n"

		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	if w.written != expected {
		t.Fatal("wrong body, got:\n", w.written, "\nexpected:\n", expected)
	}
}

func TestSendEventJson(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	var expected string

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)

		streamer.SendJSON("", "json", nil)
		expected += "event:json\ndata:null\n\n"

		streamer.SendJSON("", "json", map[string]string{"test": "successful"})
		expected += "event:json\ndata:{\"test\":\"successful\"}\n\n"

		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	if w.written != expected {
		t.Fatal("wrong body, got:\n", w.written, "\nexpected:\n", expected)
	}
}

func TestSendUInt(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	var expected string

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)

		streamer.SendUint("", "number", math.MaxUint64)
		expected += "event:number\ndata:" + strconv.FormatUint(math.MaxUint64, 10) + "\n\n"

		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	if w.written != expected {
		t.Fatal("wrong body, got:\n", w.written, "\nexpected:\n", expected)
	}
}

func TestSendInt(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	var expected string

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)

		streamer.SendInt("", "number", math.MaxInt64)
		expected += "event:number\ndata:" + strconv.FormatInt(math.MaxInt64, 10) + "\n\n"

		streamer.SendInt("", "number", math.MinInt64)
		expected += "event:number\ndata:" + strconv.FormatInt(math.MinInt64, 10) + "\n\n"
		time.Sleep(500 * time.Millisecond)

		streamer.SendInt("", "number", 5)
		expected += "event:number\ndata:" + strconv.FormatInt(5, 10) + "\n\n"
		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	if w.written != expected {
		t.Fatal("wrong body, got:\n", w.written, "\nexpected:\n", expected)
	}
}

func TestSendString(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	var expected string

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)

		streamer.SendString("", "", "")
		expected += "data:\n\n"

		streamer.SendString("", "", "Test")
		expected += "data:Test\n\n"

		streamer.SendString("", "msg", "Hi!")
		expected += "event:msg\ndata:Hi!\n\n"

		streamer.SendString("", "string", "multi\nline\n\nyay")
		expected += "event:string\ndata:multi\ndata:line\ndata:\ndata:yay\n\n"

		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	if w.written != expected {
		t.Fatal("wrong body, got:\n", w.written, "\nexpected:\n", expected)
	}
}

func TestJSONErr(t *testing.T) {
	streamer := New()
	w := NewMockResponseWriteFlushCloser()

	var expected string
	var err error

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(500 * time.Millisecond)

		// Inf can not be marshalled
		err = streamer.SendJSON("", "json", math.Inf(0))

		time.Sleep(500 * time.Millisecond)
		w.Close()
	}()

	streamer.ServeHTTP(w, nil)

	if err == nil {
		t.Fatal("expected an error!")
	}

	if w.status != http.StatusOK {
		t.Fatal("wrong status code:", w.status)
	}

	if w.written != expected {
		t.Fatal("wrong body, got:\n", w.written, "\nexpected:\n", expected)
	}
}
