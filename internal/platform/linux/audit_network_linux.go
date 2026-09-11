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
func parseSockets(b []byte, tcp bool) ([]map[string]any, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "local_address") {
		return nil, errors.New("missing socket table header")
	}
	out := []map[string]any{}
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
		out = append(out, map[string]any{"local_address": ip, "local_port": port, "remote_address": remote, "remote_port": rport, "state_hex": f[3], "uid": uid, "inode": inode})
		if len(out) > auditMaxEntries {
			return nil, errors.New("socket entry limit exceeded")
		}
	}
	return out, nil
}
func parseIPv4Routes(b []byte) ([]map[string]any, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if !strings.HasPrefix(lines[0], "Iface") {
		return nil, errors.New("missing route table header")
	}
	out := []map[string]any{}
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
		out = append(out, map[string]any{"interface": f[0], "destination": dest, "gateway": gateway, "netmask": mask, "flags": flags, "metric": metric})
		if len(out) > auditMaxEntries {
			return nil, errors.New("route entry limit exceeded")
		}
	}
	return out, nil
}
func parseIPv6Routes(b []byte) ([]map[string]any, error) {
	out := []map[string]any{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 10 {
			return nil, errors.New("invalid IPv6 route")
		}
		row := map[string]any{"interface": f[9]}
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
			row[v.k] = s
		}
		for _, v := range []struct {
			k string
			i int
		}{{"destination_prefix", 1}, {"source_prefix", 3}, {"metric", 5}, {"flags", 8}} {
			n, e := strconv.ParseUint(f[v.i], 16, 32)
			if e != nil || ((v.i == 1 || v.i == 3) && n > 128) {
				return nil, errors.New("invalid IPv6 route value")
			}
			row[v.k] = n
		}
		out = append(out, row)
		if len(out) > auditMaxEntries {
			return nil, errors.New("route entry limit exceeded")
		}
	}
	return out, nil
}
func parseInterfaces(b []byte) ([]map[string]any, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 || !strings.Contains(lines[1], "bytes") {
		return nil, errors.New("invalid interface header")
	}
	out := []map[string]any{}
	for _, line := range lines[2:] {
		name, values, ok := strings.Cut(line, ":")
		if !ok {
			return nil, errors.New("invalid interface row")
		}
		f := strings.Fields(values)
		if len(f) != 16 {
			return nil, errors.New("invalid interface counters")
		}
		row := map[string]any{"name": strings.TrimSpace(name)}
		for _, v := range []struct {
			k string
			i int
		}{{"rx_bytes", 0}, {"rx_packets", 1}, {"rx_errors", 2}, {"rx_dropped", 3}, {"tx_bytes", 8}, {"tx_packets", 9}, {"tx_errors", 10}, {"tx_dropped", 11}} {
			n, e := strconv.ParseUint(f[v.i], 10, 64)
			if e != nil {
				return nil, e
			}
			row[v.k] = n
		}
		out = append(out, row)
		if len(out) > auditMaxEntries {
			return nil, errors.New("interface limit exceeded")
		}
	}
	return out, nil
}
func parseIPv6Addresses(b []byte) ([]map[string]any, error) {
	out := []map[string]any{}
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
		row := map[string]any{"address": ip, "interface": f[5]}
		for _, v := range []struct {
			k string
			i int
		}{{"index", 1}, {"prefix", 2}, {"scope", 3}, {"flags", 4}} {
			n, e := strconv.ParseUint(f[v.i], 16, 32)
			if e != nil || (v.i == 2 && n > 128) {
				return nil, errors.New("invalid IPv6 address attribute")
			}
			row[v.k] = n
		}
		out = append(out, row)
		if len(out) > auditMaxEntries {
			return nil, errors.New("address limit exceeded")
		}
	}
	return out, nil
}
func parseIPv4Local(b []byte) ([]map[string]any, error) {
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
	out := []map[string]any{}
	for _, s := range keys {
		out = append(out, map[string]any{"address": s})
	}
	return out, nil
}
func (c *Collector) auditNetwork(ctx context.Context, r *contract.Result) {
	r.Source = "procfs network tables"
	r.Data["scope"] = "collector network namespace; non-atomic observations"
	r.Data["snapshot_consistent"] = false
	parsers := []struct {
		path, key string
		parse     func([]byte) ([]map[string]any, error)
	}{
		{"/proc/net/dev", "interfaces", parseInterfaces}, {"/proc/net/if_inet6", "ipv6_addresses", parseIPv6Addresses}, {"/proc/net/fib_trie", "ipv4_local_addresses", parseIPv4Local}, {"/proc/net/route", "ipv4_routes", parseIPv4Routes}, {"/proc/net/ipv6_route", "ipv6_routes", parseIPv6Routes},
		{"/proc/net/tcp", "tcp4_listeners", func(b []byte) ([]map[string]any, error) { return parseSockets(b, true) }}, {"/proc/net/tcp6", "tcp6_listeners", func(b []byte) ([]map[string]any, error) { return parseSockets(b, true) }}, {"/proc/net/udp", "udp4_endpoints", func(b []byte) ([]map[string]any, error) { return parseSockets(b, false) }}, {"/proc/net/udp6", "udp6_endpoints", func(b []byte) ([]map[string]any, error) { return parseSockets(b, false) }},
	}
	for _, p := range parsers {
		if ctx.Err() != nil {
			return
		}
		if c.auditBudgetExhausted() {
			r.Truncated = true
			auditIssue(r, "inspection_limit", p.path, "aggregate network input limit reached")
			break
		}
		b, ok := c.auditRead(r, p.path)
		if !ok {
			continue
		}
		rows, e := p.parse(b)
		if e != nil {
			auditIssue(r, "malformed_source", p.path, e.Error())
			continue
		}
		r.Data[p.key] = rows
	}
	auditIssue(r, "unavailable_interface", "firewall", "active firewall rules require interfaces unavailable under the existing diagnostics sandbox")
	auditIssue(r, "partial_observation", "network", "IPv4 addresses lack interface association; procfs routes omit policy routing rules and additional IPv4 tables; socket inode ownership requires separately authorized process inspection")
}
