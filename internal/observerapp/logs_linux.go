package observerapp

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

// frame header layout for the multiplexed Docker stream: one stream byte,
// three reserved bytes, and a four-byte big-endian payload size.
const frameHeaderSize = 8

var errByteCeiling = errors.New("log byte ceiling reached")

func parseDockerTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		return nil
	}
	return &t
}

// containerLogs retrieves one bounded, non-following log window and decodes
// it without treating log bytes as protocol. Rotation and driver coverage
// are reported by the caller; unsupported drivers fail explicitly.
func (c *client) containerLogs(ctx context.Context, r dockerobs.Request) dockerobs.Response {
	ctx, cancel := observeContext(ctx, observationBudget)
	defer cancel()
	maxBytes := r.Logs.MaxBytes
	if maxBytes <= 0 || maxBytes > dockerobs.MaxLogBytes {
		maxBytes = dockerobs.MaxLogBytes
	}
	api := c.negotiated
	if api == "" {
		return fail(errors.New("engine API negotiation pending"))
	}
	u := url.URL{
		Scheme:   "http",
		Host:     "docker",
		Path:     "/v" + api + "/containers/" + r.Selector + "/logs",
		RawQuery: c.fixedQuery(r).Encode(),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fail(err)
	}
	req.Host = "docker"
	req.Header.Set("User-Agent", "hostlens-docker-observer/1")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fail(context.Cause(ctx))
		}
		return fail(fmt.Errorf("engine request failed: %w", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return notFound(errEngineNotFound)
	}
	if resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusBadRequest {
		// The structured issue lets the backend report a driver gap without
		// matching error text across the IPC boundary.
		return dockerobs.Response{Failed: true, Reason: fmt.Errorf("%w: %s", errEngineUnsupported, engineErrorMessage(resp.Body)).Error(), Issue: "unsupported_driver"}
	}
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Errorf("log request rejected (%d): %s", resp.StatusCode, engineErrorMessage(resp.Body)))
	}
	contentType := resp.Header.Get("Content-Type")
	tty := strings.Contains(contentType, "raw-stream")
	page, err := decodeLogStream(resp.Body, maxBytes, tty)
	if err != nil {
		if ctx.Err() != nil {
			return fail(context.Cause(ctx))
		}
		return fail(fmt.Errorf("log stream unavailable: %w", err))
	}
	page.TTY = tty
	return dockerobs.Response{Logs: page}
}

// decodeLogStream reads one bounded log response. Non-TTY responses are
// Docker multiplexed streams; TTY responses are raw lines. Non-standard
// frames are skipped and counted, malformed content is sanitized and
// counted, and truncation is explicit.
func decodeLogStream(body io.Reader, maxBytes int, tty bool) (*dockerobs.LogPage, error) {
	page := &dockerobs.LogPage{Records: []dockerobs.LogRecord{}}
	reader := &byteBounded{r: body, remaining: int64(maxBytes)}
	if tty {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 4096), maxBytes+1024)
		for scanner.Scan() {
			if len(page.Records) >= dockerobs.MaxLogRecords {
				page.Truncated = true
				break
			}
			record, ok := timestamped(scanner.Bytes())
			if !ok {
				page.Dropped++
				continue
			}
			page.Records = append(page.Records, record)
			bumpWindow(page, record.Timestamp)
		}
		if e := scanner.Err(); e != nil && !errors.Is(e, errByteCeiling) {
			return page, e
		}
		// Only an attempted read past the ceiling reports truncation; a
		// response ending exactly at the ceiling is complete.
		if errors.Is(scanner.Err(), errByteCeiling) {
			page.Truncated = true
		}
		return page, nil
	}
	header := make([]byte, frameHeaderSize)
	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			if errors.Is(err, errByteCeiling) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				if errors.Is(err, errByteCeiling) {
					page.Truncated = true
				}
				break
			}
			return page, err
		}
		payloadSize := binary.BigEndian.Uint32(header[4:8])
		if payloadSize > uint32(maxBytes) {
			page.Truncated = true
			break
		}
		payload := make([]byte, payloadSize)
		if payloadSize > 0 {
			if _, err := io.ReadFull(reader, payload); err != nil {
				if errors.Is(err, errByteCeiling) {
					page.Truncated = true
					break
				}
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					break
				}
				return page, err
			}
		}
		if len(page.Records) >= dockerobs.MaxLogRecords {
			page.Truncated = true
			break
		}
		switch header[0] {
		case 1, 2:
			record, ok := timestamped(payload)
			if !ok {
				page.Dropped++
				continue
			}
			if header[0] == 2 {
				record.Stream = "stderr"
			}
			page.Records = append(page.Records, record)
			bumpWindow(page, record.Timestamp)
		default:
			page.Skipped++
		}
	}
	return page, nil
}

func timestamped(payload []byte) (dockerobs.LogRecord, bool) {
	line := strings.TrimSuffix(string(payload), "\n")
	line = strings.TrimSuffix(line, "\r")
	message := line
	var when *time.Time
	// A leading RFC3339 timestamp followed by one space is the daemon's
	// --timestamps=1 prefix format for both stream modes.
	if i := strings.Index(line, " "); i > 0 {
		if t, e := time.Parse(time.RFC3339Nano, line[:i]); e == nil {
			when = &t
			message = line[i+1:]
		}
	}
	if !utf8.ValidString(message) {
		message = strings.ToValidUTF8(message, string(utf8.RuneError))
	}
	return dockerobs.LogRecord{Stream: "stdout", Timestamp: when, Message: message}, when != nil || message != ""
}

// byteBounded bounds total bytes read and reports the ceiling explicitly.
// A response that ends exactly at the ceiling is complete: the exhaustion
// probe reads the underlying stream once more before declaring truncation.
type byteBounded struct {
	r         io.Reader
	remaining int64
	probed    bool
}

func (b *byteBounded) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		if b.probed {
			return 0, io.EOF
		}
		b.probed = true
		var probe [1]byte
		n, err := b.r.Read(probe[:])
		if n > 0 {
			return 0, errByteCeiling
		}
		if err == nil || errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.r.Read(p)
	b.remaining -= int64(n)
	if b.remaining <= 0 && err == nil {
		return n, nil
	}
	return n, err
}

func bumpWindow(page *dockerobs.LogPage, when *time.Time) {
	if when == nil {
		return
	}
	if page.FirstEvent == nil || when.Before(*page.FirstEvent) {
		page.FirstEvent = when
	}
	if page.LastEvent == nil || when.After(*page.LastEvent) {
		page.LastEvent = when
	}
}
