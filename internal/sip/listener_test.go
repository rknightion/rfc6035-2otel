package sip

import (
	"context"
	"net"
	"strings"
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
	listener, err := New(Config{MaxDatagram: 256, Handler: func(_ context.Context, _ Publish) { handled.Add(1) }})
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
	for _, hostile := range [][]byte{nil, []byte{0, 255, 1, 2}, []byte("PUBLISH"), make([]byte, 300)} {
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

func TestListenerRejectsWrongPublishShape(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	listener, err := New(Config{})
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
