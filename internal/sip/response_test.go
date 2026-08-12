package sip

import (
	"net"
	"strings"
	"testing"
)

func TestBuildResponse(t *testing.T) {
	request := []byte("PUBLISH sip:collector@example.test SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP phone.example.test:5060;branch=z9hG4bK-1;rport\r\n" +
		"From: <sip:phone@example.test>;tag=from-tag\r\n" +
		"To: <sip:collector@example.test>\r\n" +
		"Call-ID: call-123\r\n" +
		"CSeq: 42 PUBLISH\r\n" +
		"Event: vq-rtcpxr\r\n" +
		"Content-Type: application/vq-rtcpxr\r\n" +
		"Expires: 600\r\n\r\n")

	parsed, err := parseRequest(request)
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	response := string(buildResponse(parsed, &net.UDPAddr{IP: net.ParseIP("192.0.2.55"), Port: 43122}, "tag-1", "etag-1", 300))

	for _, want := range []string{
		"SIP/2.0 200 OK\r\n",
		"Via: SIP/2.0/UDP phone.example.test:5060;branch=z9hG4bK-1;rport=43122;received=192.0.2.55\r\n",
		"From: <sip:phone@example.test>;tag=from-tag\r\n",
		"To: <sip:collector@example.test>;tag=tag-1\r\n",
		"Call-ID: call-123\r\n",
		"CSeq: 42 PUBLISH\r\n",
		"SIP-ETag: etag-1\r\n",
		"Expires: 300\r\n",
		"Content-Length: 0\r\n",
	} {
		if !strings.Contains(response, want) {
			t.Errorf("response missing %q:\n%s", want, response)
		}
	}
}

func TestBuildResponseDoesNotExtendExpires(t *testing.T) {
	parsed, err := parseRequest([]byte("PUBLISH sip:x SIP/2.0\r\nVia: SIP/2.0/UDP phone\r\nFrom: <sip:a>;tag=f\r\nTo: <sip:b>;tag=t\r\nCall-ID: id\r\nCSeq: 1 PUBLISH\r\nExpires: 10\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	response := string(buildResponse(parsed, &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 5060}, "unused", "etag", 300))
	if !strings.Contains(response, "Expires: 10\r\n") {
		t.Fatalf("response did not cap expiry: %s", response)
	}
}
