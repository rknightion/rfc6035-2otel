// Package sip accepts RFC 6035 voice-quality PUBLISH requests over UDP.
package sip

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultDedupeWindow = 32 * time.Second
	defaultMaxDatagram  = 65535
)

// Publish is a validated SIP PUBLISH report delivered once per Call-ID/CSeq
// pair during the configured deduplication window.
type Publish struct {
	Body       []byte
	CallID     string
	CSeq       string
	RemoteAddr net.Addr
}

// Handler receives each non-duplicate, valid voice-quality PUBLISH request.
type Handler func(context.Context, Publish)

// Config configures a UDP SIP listener.
type Config struct {
	Address      string
	DedupeWindow time.Duration
	MaxDatagram  int
	Handler      Handler
}

// Listener serves SIP requests and keeps its retransmission cache safe for
// concurrent datagram handling.
type Listener struct {
	config Config
	mu     sync.Mutex
	seen   map[string]cachedResponse
}

type cachedResponse struct {
	response []byte
	expires  time.Time
}

// New creates a Listener. An empty address means UDP port 5060 on all
// interfaces. A zero deduplication window uses the required 32-second default.
func New(config Config) (*Listener, error) {
	if config.Address == "" {
		config.Address = ":5060"
	}
	if config.DedupeWindow == 0 {
		config.DedupeWindow = defaultDedupeWindow
	}
	if config.DedupeWindow < 0 {
		return nil, errors.New("sip: dedupe window must not be negative")
	}
	if config.MaxDatagram == 0 {
		config.MaxDatagram = defaultMaxDatagram
	}
	if config.MaxDatagram < 1 || config.MaxDatagram > defaultMaxDatagram {
		return nil, fmt.Errorf("sip: max datagram must be between 1 and %d", defaultMaxDatagram)
	}
	return &Listener{config: config, seen: make(map[string]cachedResponse)}, nil
}

// ListenAndServe binds the configured UDP address and serves until ctx is
// cancelled or the socket fails.
func (l *Listener) ListenAndServe(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", l.config.Address)
	if err != nil {
		return err
	}
	defer conn.Close()
	return l.Serve(ctx, conn)
}

// Serve handles datagrams from conn concurrently. It does not close conn.
func (l *Listener) Serve(ctx context.Context, conn net.PacketConn) error {
	for {
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			return err
		}
		buffer := make([]byte, l.config.MaxDatagram+1)
		n, remote, err := conn.ReadFrom(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				continue
			}
			return err
		}
		if n == len(buffer) { // The datagram may have been truncated; never parse it.
			continue
		}
		data := append([]byte(nil), buffer[:n]...)
		go l.handleDatagram(ctx, conn, remote, data)
	}
}

func (l *Listener) handleDatagram(ctx context.Context, conn net.PacketConn, remote net.Addr, data []byte) {
	request, err := parseRequest(data)
	if err != nil {
		return
	}
	status := validatePublish(request)
	if status != 200 {
		_, _ = conn.WriteTo(buildErrorResponse(request, remote, status), remote)
		return
	}

	key := request.header("call-id") + "\x00" + request.header("cseq")
	response, duplicate := l.responseFor(key, func() []byte {
		return buildResponse(request, remote, randomToken(), randomToken(), 3600)
	})
	_, _ = conn.WriteTo(response, remote)
	if !duplicate && l.config.Handler != nil {
		l.config.Handler(ctx, Publish{Body: append([]byte(nil), request.body...), CallID: request.header("call-id"), CSeq: request.header("cseq"), RemoteAddr: remote})
	}
}

func (l *Listener) responseFor(key string, build func() []byte) ([]byte, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for staleKey, entry := range l.seen {
		if !entry.expires.After(now) {
			delete(l.seen, staleKey)
		}
	}
	if entry, ok := l.seen[key]; ok {
		return entry.response, true
	}
	response := build()
	l.seen[key] = cachedResponse{response: response, expires: now.Add(l.config.DedupeWindow)}
	return response, false
}

type request struct {
	method  string
	headers map[string]string
	body    []byte
}

func (r request) header(name string) string { return r.headers[strings.ToLower(name)] }

func parseRequest(data []byte) (request, error) {
	parts := strings.SplitN(string(data), "\r\n\r\n", 2)
	if len(parts) != 2 {
		parts = strings.SplitN(string(data), "\n\n", 2)
	}
	lines := strings.Split(strings.ReplaceAll(parts[0], "\r\n", "\n"), "\n")
	if len(lines) == 0 || len(strings.Fields(lines[0])) < 3 {
		return request{}, errors.New("sip: malformed request line")
	}
	r := request{method: strings.ToUpper(strings.Fields(lines[0])[0]), headers: make(map[string]string)}
	var current string
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && current != "" {
			r.headers[current] += " " + strings.TrimSpace(line)
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return request{}, errors.New("sip: malformed header")
		}
		current = strings.ToLower(strings.TrimSpace(name))
		if _, exists := r.headers[current]; !exists {
			r.headers[current] = strings.TrimSpace(value)
		}
	}
	if len(parts) == 2 {
		r.body = []byte(parts[1])
	}
	return r, nil
}

func validatePublish(r request) int {
	if r.method != "PUBLISH" {
		return 405
	}
	if !strings.EqualFold(strings.TrimSpace(strings.Split(r.header("content-type"), ";")[0]), "application/vq-rtcpxr") {
		return 415
	}
	if !strings.EqualFold(strings.TrimSpace(strings.Split(r.header("event"), ";")[0]), "vq-rtcpxr") {
		return 489
	}
	return 200
}

func buildResponse(r request, remote net.Addr, toTag, etag string, grantExpires int) []byte {
	expires := grantedExpiry(r.header("expires"), grantExpires)
	return response(r, remote, 200, "OK", toTag, etag, expires)
}

func buildErrorResponse(r request, remote net.Addr, status int) []byte {
	reason := map[int]string{405: "Method Not Allowed", 415: "Unsupported Media Type", 489: "Bad Event"}[status]
	return response(r, remote, status, reason, "", "", -1)
}

func response(r request, remote net.Addr, status int, reason, toTag, etag string, expires int) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "SIP/2.0 %d %s\r\n", status, reason)
	if via := r.header("via"); via != "" {
		fmt.Fprintf(&b, "Via: %s\r\n", decorateVia(via, remote))
	}
	if from := r.header("from"); from != "" {
		fmt.Fprintf(&b, "From: %s\r\n", from)
	}
	if to := r.header("to"); to != "" {
		if status == 200 && !strings.Contains(strings.ToLower(to), ";tag=") {
			to += ";tag=" + toTag
		}
		fmt.Fprintf(&b, "To: %s\r\n", to)
	}
	if callID := r.header("call-id"); callID != "" {
		fmt.Fprintf(&b, "Call-ID: %s\r\n", callID)
	}
	if cseq := r.header("cseq"); cseq != "" {
		fmt.Fprintf(&b, "CSeq: %s\r\n", cseq)
	}
	if status == 200 {
		fmt.Fprintf(&b, "SIP-ETag: %s\r\nExpires: %d\r\n", etag, expires)
	}
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}

func decorateVia(via string, remote net.Addr) string {
	udp, ok := remote.(*net.UDPAddr)
	if !ok {
		return via
	}
	if !containsParam(via, "received") {
		via += ";received=" + udp.IP.String()
	}
	if containsBareParam(via, "rport") {
		via = strings.Replace(via, ";rport", ";rport="+strconv.Itoa(udp.Port), 1)
	}
	return via
}

func containsParam(value, name string) bool {
	for _, param := range strings.Split(value, ";")[1:] {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(param, "=", 2)[0]), name) {
			return true
		}
	}
	return false
}
func containsBareParam(value, name string) bool {
	for _, param := range strings.Split(value, ";")[1:] {
		if strings.EqualFold(strings.TrimSpace(param), name) {
			return true
		}
	}
	return false
}
func grantedExpiry(value string, maximum int) int {
	requested, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || requested < 0 {
		return 0
	}
	if requested < maximum {
		return requested
	}
	return maximum
}
func randomToken() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes[:])
}
