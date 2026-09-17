package linux

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/Monska85/hostlens/internal/contract"
)

func procAddress(s string, wordOrder bool) (string, error) {
	b, e := hex.DecodeString(s)
	if e != nil || (len(b) != 4 && len(b) != 16) {
		return "", errors.New("invalid network address")
	}
	if wordOrder {
		for i := 0; i < len(b); i += 4 {
			v, e := strconv.ParseUint(s[i*2:i*2+8], 16, 32)
			if e != nil {
				return "", e
			}
			binary.NativeEndian.PutUint32(b[i:i+4], uint32(v))
		}
	}
	addr, ok := netip.AddrFromSlice(b)
	if !ok {
		return "", errors.New("invalid address width")
	}
	return addr.String(), nil
}
func procEndpoint(s string) (string, uint64, error) {
	addr, port, ok := strings.Cut(s, ":")
	if !ok {
		return "", 0, errors.New("invalid socket endpoint")
	}
	ip, e := procAddress(addr, true)
	if e != nil {
		return "", 0, e
	}
	n, e := strconv.ParseUint(port, 16, 16)
	return ip, n, e
}
func parseSockets(b []byte, tcp bool) ([]contract.SocketRow, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "local_address") {
		return nil, errors.New("missing socket table header")
	}
	out := []contract.SocketRow{}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 10 {
			return nil, errors.New("short socket row")
		}
		state, e := strconv.ParseUint(f[3], 16, 8)
		if e != nil {
			return nil, e
		}
		if tcp && state != 10 {
			continue
		}
		ip, port, e := procEndpoint(f[1])
		if e != nil {
			return nil, e
		}
		remote, rport, e := procEndpoint(f[2])
		if e != nil {
			return nil, e
		}
		uid, e := strconv.ParseUint(f[7], 10, 32)
		if e != nil {
			return nil, e
		}
		inode, e := strconv.ParseUint(f[9], 10, 64)
		if e != nil {
			return nil, e
		}
		out = append(out, contract.SocketRow{LocalAddress: ip, LocalPort: port, RemoteAddress: remote, RemotePort: rport, StateHex: f[3], UID: uint32(uid), Inode: inode})
		if len(out) > auditMaxEntries {
			return nil, errors.New("socket entry limit exceeded")
		}
	}
	return out, nil
}
func parseIPv4Routes(b []byte) ([]contract.IPv4Route, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if !strings.HasPrefix(lines[0], "Iface") {
		return nil, errors.New("missing route table header")
	}
	out := []contract.IPv4Route{}
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) < 11 {
			return nil, errors.New("short IPv4 route")
		}
		dest, e := procAddress(f[1], true)
		if e != nil {
			return nil, e
		}
		gateway, e := procAddress(f[2], true)
		if e != nil {
			return nil, e
		}
		mask, e := procAddress(f[7], true)
		if e != nil {
			return nil, e
		}
		flags, e := strconv.ParseUint(f[3], 16, 32)
		if e != nil {
			return nil, e
		}
		metric, e := strconv.ParseUint(f[6], 10, 32)
		if e != nil {
			return nil, e
		}
		out = append(out, contract.IPv4Route{Interface: f[0], Destination: dest, Gateway: gateway, Netmask: mask, Flags: uint32(flags), Metric: uint32(metric)})
		if len(out) > auditMaxEntries {
			return nil, errors.New("route entry limit exceeded")
		}
	}
	return out, nil
}
func parseIPv6Routes(b []byte) ([]contract.IPv6Route, error) {
	out := []contract.IPv6Route{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 10 {
			return nil, errors.New("invalid IPv6 route")
		}
		row := contract.IPv6Route{Interface: f[9]}
		for _, v := range []struct {
			k string
			i int
		}{{"destination", 0}, {"source", 2}, {"gateway", 4}} {
			s, e := procAddress(f[v.i], false)
			if len(f[v.i]) != 32 {
				return nil, errors.New("invalid IPv6 address width")
			}
			if e != nil {
				return nil, e
			}
			switch v.k {
			case "destination":
				row.Destination = s
			case "source":
				row.Source = s
			case "gateway":
				row.Gateway = s
			}
		}
		for _, v := range []struct {
			k string
			i int
		}{{"destination_prefix", 1}, {"source_prefix", 3}, {"metric", 5}, {"flags", 8}} {
			n, e := strconv.ParseUint(f[v.i], 16, 32)
			if e != nil || ((v.i == 1 || v.i == 3) && n > 128) {
				return nil, errors.New("invalid IPv6 route value")
			}
			switch v.k {
			case "destination_prefix":
				row.DestinationPrefix = uint32(n)
			case "source_prefix":
				row.SourcePrefix = uint32(n)
			case "metric":
				row.Metric = uint32(n)
			case "flags":
				row.Flags = uint32(n)
			}
		}
		out = append(out, row)
		if len(out) > auditMaxEntries {
			return nil, errors.New("route entry limit exceeded")
		}
	}
	return out, nil
}
func parseInterfaces(b []byte) ([]contract.InterfaceRow, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 || !strings.Contains(lines[1], "bytes") {
		return nil, errors.New("invalid interface header")
	}
	out := []contract.InterfaceRow{}
	for _, line := range lines[2:] {
		name, values, ok := strings.Cut(line, ":")
		if !ok {
			return nil, errors.New("invalid interface row")
		}
		f := strings.Fields(values)
		if len(f) != 16 {
			return nil, errors.New("invalid interface counters")
		}
		row := contract.InterfaceRow{Name: strings.TrimSpace(name)}
		for _, v := range []struct {
			k string
			i int
		}{{"rx_bytes", 0}, {"rx_packets", 1}, {"rx_errors", 2}, {"rx_dropped", 3}, {"tx_bytes", 8}, {"tx_packets", 9}, {"tx_errors", 10}, {"tx_dropped", 11}} {
			n, e := strconv.ParseUint(f[v.i], 10, 64)
			if e != nil {
				return nil, e
			}
			switch v.k {
			case "rx_bytes":
				row.RxBytes = n
			case "rx_packets":
				row.RxPackets = n
			case "rx_errors":
				row.RxErrors = n
			case "rx_dropped":
				row.RxDropped = n
			case "tx_bytes":
				row.TxBytes = n
			case "tx_packets":
				row.TxPackets = n
			case "tx_errors":
				row.TxErrors = n
			case "tx_dropped":
				row.TxDropped = n
			}
		}
		out = append(out, row)
		if len(out) > auditMaxEntries {
			return nil, errors.New("interface limit exceeded")
		}
	}
	return out, nil
}
func parseIPv6Addresses(b []byte) ([]contract.IPv6Address, error) {
	out := []contract.IPv6Address{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 6 {
			return nil, errors.New("invalid IPv6 interface address")
		}
		ip, e := procAddress(f[0], false)
		if len(f[0]) != 32 {
			return nil, errors.New("invalid IPv6 address width")
		}
		if e != nil {
			return nil, e
		}
		row := contract.IPv6Address{Address: ip, Interface: f[5]}
		for _, v := range []struct {
			k string
			i int
		}{{"index", 1}, {"prefix", 2}, {"scope", 3}, {"flags", 4}} {
			n, e := strconv.ParseUint(f[v.i], 16, 32)
			if e != nil || (v.i == 2 && n > 128) {
				return nil, errors.New("invalid IPv6 address attribute")
			}
			switch v.k {
			case "index":
				row.Index = uint32(n)
			case "prefix":
				row.Prefix = uint32(n)
			case "scope":
				row.Scope = uint32(n)
			case "flags":
				row.Flags = uint32(n)
			}
		}
		out = append(out, row)
		if len(out) > auditMaxEntries {
			return nil, errors.New("address limit exceeded")
		}
	}
	return out, nil
}
func parseIPv4Local(b []byte) ([]contract.IPv4LocalAddress, error) {
	lines := strings.Split(string(b), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		if strings.TrimSpace(line) != "/32 host LOCAL" {
			continue
		}
		if i == 0 {
			return nil, errors.New("missing IPv4 local address")
		}
		prev := strings.Fields(lines[i-1])
		if len(prev) == 0 {
			return nil, errors.New("missing IPv4 local address")
		}
		addr, e := netip.ParseAddr(prev[len(prev)-1])
		if e != nil || !addr.Is4() {
			return nil, errors.New("invalid IPv4 local address")
		}
		seen[addr.String()] = true
		if len(seen) > auditMaxEntries {
			return nil, errors.New("address limit exceeded")
		}
	}
	if !strings.Contains(string(b), "Main:") && !strings.Contains(string(b), "Local:") {
		return nil, errors.New("invalid IPv4 forwarding trie")
	}
	keys := []string{}
	for s := range seen {
		keys = append(keys, s)
	}
	sort.Strings(keys)
	out := []contract.IPv4LocalAddress{}
	for _, s := range keys {
		out = append(out, contract.IPv4LocalAddress{Address: s})
	}
	return out, nil
}
func (c *Collector) auditNetwork(ctx context.Context, r *contract.Result) bool {
	p := r.Data.(*contract.NetworkInfo)
	r.Source = "procfs network tables"
	p.Scope = "collector network namespace; non-atomic observations"
	p.SnapshotConsistent = false
	evidence := false
	parsers := []struct {
		path   string
		assign func([]byte) error
	}{
		{"/proc/net/dev", func(b []byte) error {
			rows, e := parseInterfaces(b)
			if e != nil {
				return e
			}
			p.Interfaces = rows
			return nil
		}},
		{"/proc/net/if_inet6", func(b []byte) error {
			rows, e := parseIPv6Addresses(b)
			if e != nil {
				return e
			}
			p.IPv6Addresses = rows
			return nil
		}},
		{"/proc/net/fib_trie", func(b []byte) error {
			rows, e := parseIPv4Local(b)
			if e != nil {
				return e
			}
			p.IPv4LocalAddresses = rows
			return nil
		}},
		{"/proc/net/route", func(b []byte) error {
			rows, e := parseIPv4Routes(b)
			if e != nil {
				return e
			}
			p.IPv4Routes = rows
			return nil
		}},
		{"/proc/net/ipv6_route", func(b []byte) error {
			rows, e := parseIPv6Routes(b)
			if e != nil {
				return e
			}
			p.IPv6Routes = rows
			return nil
		}},
		{"/proc/net/tcp", func(b []byte) error {
			rows, e := parseSockets(b, true)
			if e != nil {
				return e
			}
			p.TCP4Listeners = rows
			return nil
		}},
		{"/proc/net/tcp6", func(b []byte) error {
			rows, e := parseSockets(b, true)
			if e != nil {
				return e
			}
			p.TCP6Listeners = rows
			return nil
		}},
		{"/proc/net/udp", func(b []byte) error {
			rows, e := parseSockets(b, false)
			if e != nil {
				return e
			}
			p.UDP4Endpoints = rows
			return nil
		}},
		{"/proc/net/udp6", func(b []byte) error {
			rows, e := parseSockets(b, false)
			if e != nil {
				return e
			}
			p.UDP6Endpoints = rows
			return nil
		}},
	}
	for _, parser := range parsers {
		if ctx.Err() != nil {
			return evidence
		}
		if c.auditBudgetExhausted() {
			r.Truncated = true
			auditIssue(r, "inspection_limit", parser.path, "aggregate network input limit reached")
			break
		}
		b, ok := c.auditRead(r, parser.path)
		if !ok {
			continue
		}
		if e := parser.assign(b); e != nil {
			auditIssue(r, "malformed_source", parser.path, e.Error())
			continue
		}
		evidence = true
	}
	auditIssue(r, "unavailable_interface", "firewall", "active firewall rules require interfaces unavailable under the existing diagnostics sandbox")
	auditIssue(r, "partial_observation", "network", "IPv4 addresses lack interface association; procfs routes omit policy routing rules and additional IPv4 tables; socket inode ownership requires separately authorized process inspection")
	return evidence
}
