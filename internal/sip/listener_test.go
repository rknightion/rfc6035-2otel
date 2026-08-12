package sip

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestListenerSurvivesHostileDatagramsAndDeduplicates(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var handled atomic.Int32
	observer := &recordingObserver{}
	listener, err := New(Config{
		MaxDatagram: 256,
		Handler:     func(_ context.Context, _ Publish) { handled.Add(1) },
		Observer:    observer,
		SenderName: func(address string) string {
			if address == "127.0.0.1" {
				return "test-phone"
			}
			return "unknown"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- listener.Serve(ctx, server) }()

	client, err := net.DialUDP("udp", nil, server.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, hostile := range [][]byte{nil, {0, 255, 1, 2}, []byte("PUBLISH"), make([]byte, 300)} {
		if _, err := client.Write(hostile); err != nil {
			t.Fatalf("send hostile input: %v", err)
		}
	}
	valid := []byte("PUBLISH sip:collector@example.test SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP phone;rport\r\nFrom: <sip:a>;tag=f\r\nTo: <sip:b>\r\n" +
		"Call-ID: call-1\r\nCSeq: 1 PUBLISH\r\nEvent: vq-rtcpxr\r\n" +
		"Content-Type: application/vq-rtcpxr\r\nExpires: 60\r\n\r\nbody")
	assertUDPResponse(t, client, valid, "SIP/2.0 200 OK")
	assertUDPResponse(t, client, valid, "SIP/2.0 200 OK")
	deadline := time.Now().Add(time.Second)
	for handled.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := handled.Load(); got != 1 {
		t.Fatalf("handler calls = %d, want 1", got)
	}
	observer.waitCount(t, "datagram:accepted", 2)
	observer.waitCount(t, "duplicate:test-phone", 1)
	observer.waitCount(t, "response:200", 2)
	observer.waitCount(t, "cache", 1)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not stop after cancellation")
	}
}

type recordingObserver struct {
	mu     sync.Mutex
	counts map[string]int
}

func (o *recordingObserver) add(key string, delta int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.counts == nil {
		o.counts = make(map[string]int)
	}
	o.counts[key] += delta
}
func (o *recordingObserver) RecordDatagram(_ context.Context, outcome string) error {
	o.add("datagram:"+outcome, 1)
	return nil
}
func (o *recordingObserver) RecordDuplicate(_ context.Context, sender string) error {
	o.add("duplicate:"+sender, 1)
	return nil
}
func (o *recordingObserver) RecordResponse(_ context.Context, status int) error {
	o.add("response:"+strconv.Itoa(status), 1)
	return nil
}
func (o *recordingObserver) RecordDedupeCacheChange(_ context.Context, delta int64) {
	o.add("cache", int(delta))
}
func (o *recordingObserver) waitCount(t *testing.T, key string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		o.mu.Lock()
		got := o.counts[key]
		all := make(map[string]int, len(o.counts))
		for name, count := range o.counts {
			all[name] = count
		}
		o.mu.Unlock()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s = %d, want %d (all: %#v)", key, got, want, all)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestListenerRejectsWrongPublishShape(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	observer := &recordingObserver{}
	listener, err := New(Config{Observer: observer})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go listener.Serve(ctx, server)
	client, err := net.DialUDP("udp", nil, server.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	base := "Via: SIP/2.0/UDP phone\r\nFrom: <sip:a>;tag=f\r\nTo: <sip:b>\r\nCall-ID: id\r\nCSeq: 1 PUBLISH\r\nContent-Type: application/vq-rtcpxr\r\nEvent: vq-rtcpxr\r\n\r\n"
	for _, test := range []struct{ request, want string }{
		{"OPTIONS sip:x SIP/2.0\r\n" + base, "405"},
		{"PUBLISH sip:x SIP/2.0\r\n" + strings.Replace(base, "application/vq-rtcpxr", "text/plain", 1), "415"},
		{"PUBLISH sip:x SIP/2.0\r\n" + strings.Replace(base, "Event: vq-rtcpxr", "Event: presence", 1), "489"},
	} {
		assertUDPResponse(t, client, []byte(test.request), "SIP/2.0 "+test.want)
	}
	observer.waitCount(t, "datagram:rejected", 3)
	observer.waitCount(t, "response:405", 1)
	observer.waitCount(t, "response:415", 1)
	observer.waitCount(t, "response:489", 1)
}

func assertUDPResponse(t *testing.T, client *net.UDPConn, request []byte, want string) {
	t.Helper()
	if _, err := client.Write(request); err != nil {
		t.Fatal(err)
	}
	if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 2048)
	n, err := client.Read(buffer)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if response := string(buffer[:n]); !strings.Contains(response, want) {
		t.Fatalf("response %q does not contain %q", response, want)
	}
}
