package linux

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

func TestProcNetworkAddressOrder(t *testing.T) {
	t.Parallel()

	// proc socket addresses print each native u32 as hexadecimal, including IPv6.
	ipv4 := "0100007F"
	ipv6 := "00000000000000000000000001000000"
	if binary.NativeEndian.Uint32([]byte{1, 0, 0, 0}) != 1 {
		ipv4 = "7F000001"
		ipv6 = "00000000000000000000000000000001"
	}
	for _, tc := range []struct {
		in   string
		word bool
		want string
	}{{ipv4, true, "127.0.0.1"}, {ipv6, true, "::1"}, {"20010db8000000000000000000000001", false, "2001:db8::1"}} {
		got, e := procAddress(tc.in, tc.word)
		if e != nil || got != tc.want {
			t.Fatalf("%q: %s %v", tc.in, got, e)
		}
	}
	for _, bad := range []string{"GG000000", "1234", "", "0000000000000000000000000000000000"} {
		if _, e := procAddress(bad, true); e == nil {
			t.Fatal(bad)
		}
	}
}
func TestSocketFilteringAndMalformedRows(t *testing.T) {
	t.Parallel()

	header := "sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode\n"
	listener := "0: 00000000:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 987\n"
	connected := "1: 00000000:1234 00000000:4321 01 00000000:00000000 00:00000000 00000000 1000 0 988\n"
	rows, e := parseSockets([]byte(header+listener+connected), true)
	if e != nil || len(rows) != 1 || rows[0].LocalPort != 5432 || rows[0].Inode != 987 {
		t.Fatalf("%v %v", rows, e)
	}
	rows, e = parseSockets([]byte(header+listener+connected), false)
	if e != nil || len(rows) != 2 {
		t.Fatalf("%v %v", rows, e)
	}
	for _, bad := range []string{"", header + "0: short", header + "0: INVALID 00000000:0000 0A 0 0 0 1000 0 42"} {
		if _, e := parseSockets([]byte(bad), true); e == nil {
			t.Fatal(bad)
		}
	}
}
func TestNativeNetworkParsers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, good, bad string
		parse           func([]byte) (int, error)
	}{
		{"interfaces", "Inter-| Receive | Transmit\nface |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\nlo: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16\n", "bad", func(b []byte) (int, error) { rows, e := parseInterfaces(b); return len(rows), e }},
		{"ipv4 routes", "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 00000000 0003 0 0 100 00000000 0 0 0\n", "Iface\nshort", func(b []byte) (int, error) { rows, e := parseIPv4Routes(b); return len(rows), e }},
		{"ipv6 routes", "00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000001 00000001 00000000 00000000 00000003 lo\n", "0000 00", func(b []byte) (int, error) { rows, e := parseIPv6Routes(b); return len(rows), e }},
		{"ipv6 addresses", "00000000000000000000000000000001 01 80 10 80 lo\n", "00000000000000000000000000000001 01 FF 10 80 lo", func(b []byte) (int, error) { rows, e := parseIPv6Addresses(b); return len(rows), e }},
		{"ipv4 addresses", "Main:\n +-- 127.0.0.1\n /32 host LOCAL\nLocal:\n +-- 127.0.0.1\n /32 host LOCAL\n", "Main:\n +-- broken\n /32 host LOCAL", func(b []byte) (int, error) { rows, e := parseIPv4Local(b); return len(rows), e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			count, e := tc.parse([]byte(tc.good))
			if e != nil || count != 1 {
				t.Fatalf("%d %v", count, e)
			}
			if _, e := tc.parse([]byte(tc.bad)); e == nil {
				t.Fatal("accepted malformed source")
			}
		})
	}
}
func TestNetworkCoverageDenialsAndBounds(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	writeAuditFixture(t, c, "/proc/net/tcp", "sl local_address\n0: 00000000:1538 00000000:0000 0A 0 0 0 1000 0 987\n")
	r := auditTestResult("get_network_info")
	c.auditNetwork(context.Background(), &r)
	network, ok := r.Data.(*contract.NetworkInfo)
	if !ok || auditCoverageOf(r) || len(network.TCP4Listeners) != 1 {
		t.Fatal(r)
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/net/tcp", Deny: true})
	r = auditTestResult("get_network_info")
	c.auditNetwork(context.Background(), &r)
	if r.Data.(*contract.NetworkInfo).TCP4Listeners != nil {
		t.Fatal("denied sockets exposed")
	}
}

func TestNetworkOversizedSourceStopsFurtherCollection(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	writeAuditFixture(t, c, "/proc/net/dev", strings.Repeat("x", 65))
	writeAuditFixture(t, c, "/proc/net/if_inet6", "00000000000000000000000000000001 01 80 10 80 lo\n")
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/net/if_inet6", Deny: true})
	budget := 64
	c.auditRemaining = &budget
	r := auditTestResult("get_network_info")
	c.auditNetwork(context.Background(), &r)
	network := r.Data.(*contract.NetworkInfo)
	if !r.Truncated || network.Interfaces != nil || network.IPv6Addresses != nil || budget > 0 {
		t.Fatal(r)
	}
	for _, issue := range r.Issues {
		if issue.Code == "policy_denied" {
			t.Fatal("attempted later source after exhausting input budget", r)
		}
	}
}

func TestNetworkTypedPayloadCollectsEverySource(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	socketHeader := "sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode\n"
	for _, tc := range []struct{ path, content string }{
		{"/proc/net/dev", "Inter-|   Receive                                                |  Transmit\n" +
			" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n" +
			"  lo:       100       10    0    0    0     0          0         0     200       20    0    0    0     0       0          0\n" +
			"eth0:       111       11    1    2    0     0          0         0     222       22    3    4    0     0       0          0\n"},
		{"/proc/net/if_inet6", "00000000000000000000000000000001 01 80 10 80 lo\n"},
		{"/proc/net/fib_trie", "Main:\n   |-- 10.0.0.5\n             /32 host LOCAL\n"},
		{"/proc/net/route", "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 00000000 0003 0 0 100 00000000 0 0 0\n"},
		{"/proc/net/ipv6_route", "00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000001 00000001 00000000 00000000 00000003 lo\n"},
		{"/proc/net/tcp", socketHeader + "0: 00000000:1538 00000000:0000 0A 0 0 0 1000 0 987\n"},
		{"/proc/net/tcp6", socketHeader + "0: 00000000000000000000000000000001:1538 00000000000000000000000000000000:0000 0A 0 0 0 1000 0 988\n"},
		{"/proc/net/udp", socketHeader + "0: 00000000:0035 00000000:0000 07 0 0 0 1000 0 42\n"},
		{"/proc/net/udp6", socketHeader + "0: 00000000000000000000000000000001:0035 00000000000000000000000000000000:0000 07 0 0 0 1000 0 43\n"},
	} {
		writeAuditFixture(t, c, tc.path, tc.content)
	}
	r := auditTestResult("get_network_info")
	if !c.auditNetwork(context.Background(), &r) {
		t.Fatal("network audit collected no evidence", r.Issues)
	}
	n, ok := r.Data.(*contract.NetworkInfo)
	if !ok {
		t.Fatalf("typed payload lost: %T", r.Data)
	}
	if len(n.Interfaces) != 2 || n.Interfaces[1].Name != "eth0" || n.Interfaces[1].RxBytes != 111 || n.Interfaces[1].TxDropped != 4 {
		t.Fatalf("interfaces: %+v", n.Interfaces)
	}
	if len(n.IPv6Addresses) != 1 || n.IPv6Addresses[0].Interface != "lo" || n.IPv6Addresses[0].Prefix != 128 {
		t.Fatalf("ipv6 addresses: %+v", n.IPv6Addresses)
	}
	if len(n.IPv4LocalAddresses) != 1 || n.IPv4LocalAddresses[0].Address != "10.0.0.5" {
		t.Fatalf("ipv4 local: %+v", n.IPv4LocalAddresses)
	}
	if len(n.IPv4Routes) != 1 || n.IPv4Routes[0].Metric != 100 {
		t.Fatalf("ipv4 routes: %+v", n.IPv4Routes)
	}
	if len(n.IPv6Routes) != 1 || n.IPv6Routes[0].Interface != "lo" || n.IPv6Routes[0].Metric != 1 || n.IPv6Routes[0].Flags != 3 {
		t.Fatalf("ipv6 routes: %+v", n.IPv6Routes)
	}
	if len(n.TCP4Listeners) != 1 || n.TCP4Listeners[0].Inode != 987 || n.TCP4Listeners[0].LocalPort != 5432 {
		t.Fatalf("tcp4: %+v", n.TCP4Listeners)
	}
	if len(n.TCP6Listeners) != 1 || len(n.UDP4Endpoints) != 1 || n.UDP4Endpoints[0].Inode != 42 || len(n.UDP6Endpoints) != 1 {
		t.Fatalf("socket rows: %+v %+v %+v", n.TCP6Listeners, n.UDP4Endpoints, n.UDP6Endpoints)
	}
}
