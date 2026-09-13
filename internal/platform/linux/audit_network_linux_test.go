package linux

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

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
	if e != nil || len(rows) != 1 || rows[0]["local_port"] != uint64(5432) || rows[0]["inode"] != uint64(987) {
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
		parse           func([]byte) ([]map[string]any, error)
	}{
		{"interfaces", "Inter-| Receive | Transmit\nface |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\nlo: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16\n", "bad", parseInterfaces},
		{"ipv4 routes", "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 00000000 0003 0 0 100 00000000 0 0 0\n", "Iface\nshort", parseIPv4Routes},
		{"ipv6 routes", "00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000001 00000001 00000000 00000000 00000003 lo\n", "0000 00", parseIPv6Routes},
		{"ipv6 addresses", "00000000000000000000000000000001 01 80 10 80 lo\n", "00000000000000000000000000000001 01 FF 10 80 lo", parseIPv6Addresses},
		{"ipv4 addresses", "Main:\n +-- 127.0.0.1\n /32 host LOCAL\nLocal:\n +-- 127.0.0.1\n /32 host LOCAL\n", "Main:\n +-- broken\n /32 host LOCAL", parseIPv4Local},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, e := tc.parse([]byte(tc.good))
			if e != nil || len(rows) != 1 {
				t.Fatalf("%v %v", rows, e)
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
	r := auditTestResult()
	c.auditNetwork(context.Background(), &r)
	if r.Data["coverage_complete"] != false || len(r.Data["tcp4_listeners"].([]map[string]any)) != 1 {
		t.Fatal(r)
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/net/tcp", Deny: true})
	r = auditTestResult()
	c.auditNetwork(context.Background(), &r)
	if r.Data["tcp4_listeners"] != nil {
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
	r := auditTestResult()
	c.auditNetwork(context.Background(), &r)
	if !r.Truncated || r.Data["interfaces"] != nil || r.Data["ipv6_addresses"] != nil || budget > 0 {
		t.Fatal(r)
	}
	for _, issue := range r.Issues {
		if issue.Code == "policy_denied" {
			t.Fatal("attempted later source after exhausting input budget", r)
		}
	}
}
