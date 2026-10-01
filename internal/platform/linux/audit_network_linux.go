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
		var addresses [3]string
		for i, field := range []int{0, 2, 4} {
			s, e := procAddress(f[field], false)
			if len(f[field]) != 32 {
				return nil, errors.New("invalid IPv6 address width")
			}
			if e != nil {
				return nil, e
			}
			addresses[i] = s
		}
		var values [4]uint32
		for i, field := range []int{1, 3, 5, 8} {
			n, e := strconv.ParseUint(f[field], 16, 32)
			if e != nil || (i < 2 && n > 128) {
				return nil, errors.New("invalid IPv6 route value")
			}
			values[i] = uint32(n)
		}
		row := contract.IPv6Route{
			Interface: f[9], Destination: addresses[0], Source: addresses[1], Gateway: addresses[2],
			DestinationPrefix: values[0], SourcePrefix: values[1], Metric: values[2], Flags: values[3],
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
		var counters [8]uint64
		for i, field := range []int{0, 1, 2, 3, 8, 9, 10, 11} {
			n, e := strconv.ParseUint(f[field], 10, 64)
			if e != nil {
				return nil, e
			}
			counters[i] = n
		}
		row := contract.InterfaceRow{
			Name: strings.TrimSpace(name), RxBytes: counters[0], RxPackets: counters[1], RxErrors: counters[2], RxDropped: counters[3],
			TxBytes: counters[4], TxPackets: counters[5], TxErrors: counters[6], TxDropped: counters[7],
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
		var values [4]uint32
		for i, field := range []int{1, 2, 3, 4} {
			n, e := strconv.ParseUint(f[field], 16, 32)
			if e != nil || (i == 1 && n > 128) {
				return nil, errors.New("invalid IPv6 address attribute")
			}
			values[i] = uint32(n)
		}
		row := contract.IPv6Address{Address: ip, Interface: f[5], Index: values[0], Prefix: values[1], Scope: values[2], Flags: values[3]}
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
