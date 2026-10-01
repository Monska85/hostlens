package identity

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileIDBoundsAndRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "passwd")
	for _, tc := range []struct {
		name    string
		content string
		lookup  string
		want    uint32
		missing bool
		bad     bool
	}{
		{name: "normal", content: "root:x:0:0:root:/root:/bin/sh\napp:x:1234:1234::/srv:/bin/false\n", lookup: "app", want: 1234},
		{name: "missing", content: "root:x:0:0:root:/root:/bin/sh\n", lookup: "app", missing: true},
		{name: "invalid id", content: "app:x:4294967296:1::/srv:/bin/false\n", lookup: "app", bad: true},
		{name: "oversized", content: strings.Repeat("x", MaxAccountFileBytes+1), lookup: "app", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := FileID(path, tc.lookup)
			if tc.missing && !errors.Is(err, ErrNameNotFound) {
				t.Fatalf("missing identity error: %v", err)
			}
			if tc.bad && err == nil {
				t.Fatal("invalid identity accepted")
			}
			if !tc.missing && !tc.bad && (err != nil || got != tc.want) {
				t.Fatalf("identity: got %d, err %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestPeerUIDRequiresUnixConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := PeerUID(client); err == nil {
		t.Fatal("TCP connection accepted as Unix peer")
	}
}
