package containers

import (
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type staticTransport struct{ body string }

func (s staticTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(s.body)), Header: http.Header{}}, nil
}

func TestObserverTransportHasOnlyNamedReads(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, path := range []string{"/v1.56/containers/" + id + "/json", "/v1.56/containers/" + id + "/stats", "/v1.56/containers/" + id + "/logs"} {
		if !allowedPath(http.MethodGet, path, false) {
			t.Fatalf("named read denied: %s", path)
		}
	}
	for _, path := range []string{"/v1.56/containers/" + id + "/restart", "/v1.56/containers/" + id + "/exec", "/v1.56/containers/short/logs", "/v1.56/images/json"} {
		if allowedPath(http.MethodGet, path, false) || allowedPath(http.MethodPost, path, false) {
			t.Fatalf("observer accepted outside operation: %s", path)
		}
	}
	if !allowedPath(http.MethodPost, "/v1.56/containers/"+id+"/restart", true) {
		t.Fatal("repair restart denied")
	}
}

func TestLogCollectorStopsAtByteAndChunkCeilings(t *testing.T) {
	collector := &logCollector{}
	writer := logWriter{collector: collector, stream: "stdout"}
	input := []byte(strings.Repeat("x", (64<<10)+10))
	n, err := writer.Write(input)
	if n != 64<<10 || !errors.Is(err, errLogLimit) || collector.used != 64<<10 {
		t.Fatalf("byte limit: %d, %v, %d", n, err, collector.used)
	}
	if _, err := writer.Write([]byte("again")); !errors.Is(err, errLogLimit) {
		t.Fatal("collector accepted data after byte ceiling")
	}
	collector = &logCollector{chunks: make([]LogChunk, 200)}
	writer = logWriter{collector: collector, stream: "stderr"}
	if _, err := writer.Write([]byte("again")); !errors.Is(err, errLogLimit) {
		t.Fatal("collector accepted data after chunk ceiling")
	}
}

func TestDockerLogResponseHasSmallerWireCeiling(t *testing.T) {
	id := strings.Repeat("a", 64)
	request, err := http.NewRequest(http.MethodGet, "http://docker.local/v1.56/containers/"+id+"/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := cappedTransport{base: staticTransport{body: strings.Repeat("x", (512<<10)+1)}}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if len(data) != 512<<10 || !errors.Is(err, errLogLimit) {
		t.Fatalf("bounded log prefix and truncation signal: %d, %v", len(data), err)
	}
}

func TestLogCountCeilingDropsOldestEntry(t *testing.T) {
	chunks, truncated := keepLatestLines([]LogChunk{{Stream: "stdout", Text: "first\nsecond\n"}, {Stream: "stderr", Text: "third\n"}}, 2)
	if !truncated || len(chunks) != 2 || chunks[0].Text != "second\n" || chunks[1].Text != "third\n" {
		t.Fatalf("count ceiling: %+v, %t", chunks, truncated)
	}
}

func TestDockerLogFrameSizeIsCheckedBeforeDemultiplexing(t *testing.T) {
	frame := make([]byte, 8)
	frame[0] = 1
	binary.BigEndian.PutUint32(frame[4:], 1<<30)
	if validDockerFrames(frame) {
		t.Fatal("frame could trigger a large allocation")
	}
	frame = append(frame, []byte("ok")...)
	binary.BigEndian.PutUint32(frame[4:], 2)
	if !validDockerFrames(frame) {
		t.Fatal("bounded frame rejected")
	}
}
