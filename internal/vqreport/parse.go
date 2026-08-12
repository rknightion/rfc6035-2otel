package vqreport

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// Dialect is the wire grammar identified from a report body.
// Keep all dialect choices in this package so a future dialect is a local change.
type Dialect uint8

const (
	Unknown Dialect = iota
	Standard
	// Prestandard is reserved for the measured Poly pre-standard grammar.
	// It is not detected until a real capture defines that grammar.
	Prestandard
)

func (d Dialect) String() string {
	switch d {
	case Standard:
		return "standard"
	case Prestandard:
		return "prestandard"
	default:
		return "unknown"
	}
}

var (
	// ErrUnrecognizedDialect means the body has no structurally recognisable RFC 6035 report line.
	ErrUnrecognizedDialect = errors.New("vqreport: unrecognized or unmeasured dialect")
	ErrInvalidInput        = errors.New("vqreport: input must be string or []byte")
)

// Field preserves one parsed header or NAME=value token. Key is the original
// group/key spelling (for example, Signal.RERL); Value is the unmodified value
// except for separator whitespace; Line is the one-based logical line number.
type Field struct {
	Key, Value string
	Line       int
}

// Address is a typed RTP endpoint. Each optional component is nil when omitted
// or malformed, while its original text remains available in Fields.
type Address struct {
	IP   netip.Addr
	Port *uint16
	SSRC *uint32
}

// Metrics contains supported typed observations from a local or remote metric block.
// Unrecognised observations remain in Report.Fields and Report.Unknown.
type Metrics struct {
	PayloadType                                                    *uint8
	Codec                                                          *string
	PacketsPerSecond                                               *float64
	MOSLQ, MOSCQ, RLQ, RCQ                                         *float64
	PacketLoss, DiscardRate, RoundTripDelay, OneWayDelay, IAJ, MAJ *float64
	RERL                                                           *float64
}

// Report is a lossless parsed VQ report plus convenient typed values.
type Report struct {
	Dialect                           Dialect
	ReportType                        string
	ReportDisposition                 string
	CallID, LocalID, RemoteID, OrigID string
	LocalAddress, RemoteAddress       *Address
	LocalMetrics, RemoteMetrics       Metrics
	// RawLines retains physical input lines, including folded continuations.
	RawLines []string
	// Fields retains every parsed header and metric token, including unknowns.
	Fields []Field
	// Unknown is indexed by normalised field path. Its slices retain the
	// original spelling and every occurrence, including normalisation collisions.
	Unknown map[string][]Field
}

var tokenBoundary = regexp.MustCompile(`(?:^|\s)([A-Za-z][A-Za-z0-9_-]*)\s*=`)

// Parse parses a string or byte slice report body. Standard dialect detection
// is structural: the first non-empty logical line must be a VQ*Report type.
func Parse(input any) (Report, error) {
	var body string
	switch v := input.(type) {
	case string:
		body = v
	case []byte:
		body = string(v)
	default:
		return Report{}, ErrInvalidInput
	}
	logical := foldLines(body)
	if len(logical) == 0 {
		return Report{}, ErrUnrecognizedDialect
	}
	report := Report{RawLines: strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n"), Unknown: make(map[string][]Field)}
	first := -1
	for i, line := range logical {
		if strings.TrimSpace(line.text) != "" {
			first = i
			break
		}
	}
	if first < 0 {
		return Report{}, ErrUnrecognizedDialect
	}
	typ, disposition, ok := reportLine(logical[first].text)
	if !ok {
		return Report{}, ErrUnrecognizedDialect
	}
	report.Dialect, report.ReportType, report.ReportDisposition = Standard, typ, disposition
	report.Fields = append(report.Fields, Field{Key: typ, Value: disposition, Line: logical[first].line})
	local := true
	for _, line := range logical[first+1:] {
		if strings.TrimSpace(line.text) == "" {
			continue
		}
		key, value, ok := splitLine(line.text)
		if !ok {
			report.addUnknown(Field{Key: strings.TrimSpace(line.text), Line: line.line})
			continue
		}
		canonical := normal(key)
		field := Field{Key: key, Value: value, Line: line.line}
		switch canonical {
		case "callid":
			report.CallID = value
			report.Fields = append(report.Fields, field)
		case "localid":
			report.LocalID = value
			report.Fields = append(report.Fields, field)
		case "remoteid":
			report.RemoteID = value
			report.Fields = append(report.Fields, field)
		case "origid":
			report.OrigID = value
			report.Fields = append(report.Fields, field)
		case "localaddr":
			report.LocalAddress = parseAddress(&report, key, value, line.line)
		case "remoteaddr":
			report.RemoteAddress = parseAddress(&report, key, value, line.line)
		case "localmetrics", "metrics":
			local = true
			report.Fields = append(report.Fields, field)
		case "remotemetrics":
			local = false
			report.Fields = append(report.Fields, field)
		default:
			if tokenBoundary.MatchString(value) {
				parseMetricLine(&report, key, value, line.line, local)
			} else {
				report.Fields = append(report.Fields, field)
				report.addUnknown(field)
			}
		}
	}
	return report, nil
}

type logicalLine struct {
	text string
	line int
}

func foldLines(body string) []logicalLine {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	result := make([]logicalLine, 0, len(lines))
	for n, line := range lines {
		if len(result) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			result[len(result)-1].text += " " + strings.TrimSpace(line)
		} else {
			result = append(result, logicalLine{text: line, line: n + 1})
		}
	}
	return result
}
func reportLine(line string) (string, string, bool) {
	key, value, colon := strings.Cut(strings.TrimSpace(line), ":")
	if !colon {
		key, value = strings.TrimSpace(line), ""
	}
	switch key {
	case "VQSessionReport", "VQIntervalReport", "VQAlertReport":
		return key, strings.TrimSpace(value), true
	}
	return "", "", false
}
func splitLine(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, ":")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}
func normal(s string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, s))
}
func (r *Report) addUnknown(field Field) {
	k := normal(field.Key)
	r.Unknown[k] = append(r.Unknown[k], field)
}

func parseAddress(r *Report, group, value string, line int) *Address {
	a := &Address{}
	tokens := parseTokens(value)
	for _, token := range tokens {
		field := Field{Key: group + "." + token.name, Value: token.value, Line: line}
		r.Fields = append(r.Fields, field)
		switch normal(token.name) {
		case "ip":
			if ip, err := netip.ParseAddr(token.value); err == nil {
				a.IP = ip
			} else {
				r.addUnknown(field)
			}
		case "port":
			if n, err := uintValue(token.value, 16); err == nil {
				v := uint16(n)
				a.Port = &v
			} else {
				r.addUnknown(field)
			}
		case "ssrc":
			if n, err := uintValue(token.value, 32); err == nil {
				v := uint32(n)
				a.SSRC = &v
			} else {
				r.addUnknown(field)
			}
		default:
			r.addUnknown(field)
		}
	}
	return a
}

type token struct{ name, value string }

func parseTokens(value string) []token {
	matches := tokenBoundary.FindAllStringSubmatchIndex(value, -1)
	result := make([]token, 0, len(matches))
	for i, match := range matches {
		end := len(value)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		name := value[match[2]:match[3]]
		eq := strings.IndexByte(value[match[0]:match[1]], '=')
		start := match[0] + eq + 1
		result = append(result, token{name, strings.TrimSpace(value[start:end])})
	}
	return result
}
func parseMetricLine(r *Report, group, value string, line int, local bool) {
	metrics := &r.RemoteMetrics
	if local {
		metrics = &r.LocalMetrics
	}
	for _, token := range parseTokens(value) {
		field := Field{Key: group + "." + token.name, Value: token.value, Line: line}
		r.Fields = append(r.Fields, field)
		if !setMetric(metrics, token.name, token.value) {
			r.addUnknown(field)
		}
	}
}
func setMetric(m *Metrics, name, value string) bool {
	if normal(name) == "pd" {
		v := value
		m.Codec = &v
		return true
	}
	if normal(name) == "pt" {
		n, err := uintValue(value, 8)
		if err != nil {
			return false
		}
		v := uint8(n)
		m.PayloadType = &v
		return true
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}
	if normal(name) == "rerl" && n == 127 {
		return true
	}
	switch normal(name) {
	case "pps":
		m.PacketsPerSecond = &n
	case "moslq":
		m.MOSLQ = &n
	case "moscq":
		m.MOSCQ = &n
	case "rlq":
		m.RLQ = &n
	case "rcq":
		m.RCQ = &n
	case "nlr":
		m.PacketLoss = &n
	case "jdr":
		m.DiscardRate = &n
	case "rtd":
		m.RoundTripDelay = &n
	case "sowd":
		m.OneWayDelay = &n
	case "iaj":
		m.IAJ = &n
	case "maj":
		m.MAJ = &n
	case "rerl":
		m.RERL = &n
	default:
		return false
	}
	return true
}
func uintValue(value string, bits int) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", err, value)
	}
	return n, nil
}
