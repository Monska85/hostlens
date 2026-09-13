package dockerobs

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRequestValidation(t *testing.T) {
	idHex := "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"
	valid := []Request{
		{Version: ProtocolVersion, Operation: OpEngineInfo},
		{Version: ProtocolVersion, Operation: OpContainerList},
		{Version: ProtocolVersion, Operation: OpContainerDetail, Selector: "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"},
		{Version: ProtocolVersion, Operation: OpContainerLogs, Selector: "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f", Logs: LogOptions{Records: 10, MaxBytes: 1024}},
		{Version: ProtocolVersion, Operation: OpDiskUsage},
	}
	for _, r := range valid {
		if e := r.Validate(); e != nil {
			t.Fatalf("valid request rejected: %v", e)
		}
	}
	invalid := []Request{
		{Version: 2, Operation: OpEngineInfo},
		{Version: ProtocolVersion, Operation: "mutation_op"},
		{Version: ProtocolVersion, Operation: OpContainerDetail},
		{Version: ProtocolVersion, Operation: OpContainerDetail, Selector: "web"},
		{Version: ProtocolVersion, Operation: OpContainerList, Selector: "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"},
		{Version: ProtocolVersion, Operation: OpContainerLogs, Logs: LogOptions{Since: time.Now(), Until: time.Now().Add(-time.Hour)}},
		{Version: ProtocolVersion, Operation: OpContainerLogs, Logs: LogOptions{MaxBytes: MaxLogBytes + 1}},
		{Version: ProtocolVersion, Operation: OpContainerDetail, Selector: idHex, Logs: LogOptions{Records: -1}},
		{Version: ProtocolVersion, Operation: OpContainerStats, Selector: idHex, Logs: LogOptions{Records: MaxLogRecords + 1}},
		{Version: ProtocolVersion, Operation: OpContainerStats, Selector: idHex, Logs: LogOptions{MaxBytes: -1}},
		{Version: ProtocolVersion, Operation: OpContainerLogs, Selector: idHex, Logs: LogOptions{MaxBytes: MaxLogBytes + 1}},
		{Version: ProtocolVersion, Operation: OpContainerLogs, Selector: idHex, Logs: LogOptions{Since: time.Now(), Until: time.Now().Add(-time.Hour)}},
	}
	if !ValidID(idHex) || ValidID("web") {
		t.Fatal("ValidID must accept only full hex identities")
	}
	for _, r := range invalid {
		if e := r.Validate(); e == nil {
			t.Fatalf("invalid request accepted: %+v", r)
		}
	}
}

func TestNegotiateRange(t *testing.T) {
	for _, tc := range []struct {
		daemon, daemonMin, max, want string
		wantErr                      bool
	}{
		{"1.51", "1.41", APICeiling, "1.51", false},
		{"1.56", "1.41", APICeiling, APICeiling, false},
		{"1.41", "1.41", APICeiling, "1.41", false},
		{"1.40", "1.41", APICeiling, "", true},
		{"1.41", "1.40", APICeiling, "1.41", false},
		{"1.56", "1.40", APICeiling, APICeiling, false},
		{"1.56", "1.55", APICeiling, "", true},
		{"1.56", "1.53", APICeiling, "", true},
		{"1.44", "1.44", APICeiling, "1.44", false},
		{"garbage", "1.41", APICeiling, "", true},
		{"1.41", "", APICeiling, "1.41", false},
		{"1.41", "garbage", APICeiling, "", true},
		{"1.41", "1.41", "garbage", "", true},
		{"1.41", "1.60", APICeiling, "", true},
		{"1.41", "1.42", APICeiling, "", true},
	} {
		got, e := Negotiate(tc.daemon, tc.daemonMin, tc.max)
		if (e != nil) != tc.wantErr {
			t.Fatalf("negotiate(%s,%s): %v", tc.daemon, tc.daemonMin, e)
		}
		if !tc.wantErr && got != tc.want {
			t.Fatalf("negotiate(%s) = %s, want %s", tc.daemon, got, tc.want)
		}
	}
}

func TestAPICVersionParsing(t *testing.T) {
	for _, s := range []string{"1.41", "v1.41", "1.51"} {
		if _, e := APIVersion(s); e != nil {
			t.Fatalf("parse %q: %v", s, e)
		}
	}
	for _, s := range []string{"", "1", "1.", "1.41.1", "x.y", "-1.41"} {
		if _, e := APIVersion(s); e == nil {
			t.Fatalf("accepted invalid version %q", s)
		}
	}
	if v, e := APIVersion("1.51"); e != nil || v != 151 {
		t.Fatalf("version value %d %v", v, e)
	}
}

func TestStableFingerprintEquality(t *testing.T) {
	a := StableFingerprint{ContainerIDs: []string{"b", "a"}, References: map[string]string{"a": "x", "b": "y"}}
	b := StableFingerprint{ContainerIDs: []string{"a", "b"}, References: map[string]string{"b": "y", "a": "x"}}
	if !a.Equal(b) {
		t.Fatal("order-insensitive fingerprints must be equal")
	}
	b.References["a"] = "z"
	if a.Equal(b) {
		t.Fatal("reference change must change the fingerprint")
	}
	if a.Equal(StableFingerprint{ContainerIDs: []string{"a"}}) {
		t.Fatal("container count mismatch must not be equal")
	}
	if a.Equal(StableFingerprint{ContainerIDs: []string{"a", "b"}, References: map[string]string{"a": "x"}}) {
		t.Fatal("reference count mismatch must not be equal")
	}
	if a.Equal(StableFingerprint{ContainerIDs: []string{"a", "c"}, References: map[string]string{"a": "x", "b": "y"}}) {
		t.Fatal("container identity mismatch must not be equal")
	}
}

func TestResponseExcludesUnknownFields(t *testing.T) {
	// A future daemon field must not automatically reach clients: decode a
	// summary with unknown keys and verify the projection omits them.
	var summary ContainerSummary
	payload := `{"id":"abc","names":["web"],"image_id":"sha256:x","state":"running","sneaky_future_field":"leak"}`
	if e := json.Unmarshal([]byte(payload), &summary); e != nil {
		t.Fatal(e)
	}
	data := ContainerData(summary)
	for key := range data {
		switch key {
		case "id", "names", "image_id", "state", "image", "status", "health", "created", "started_at", "finished_at", "restart_count", "ports", "mounts", "networks", "log_driver", "restart_policy", "pids_limit", "nano_cpus", "cpu_shares", "cpu_period", "cpu_quota", "memory_limit", "memory_swap":
		default:
			t.Fatalf("projection emitted unmaintained key %s", key)
		}
	}
	if _, ok := data["sneaky_future_field"]; ok {
		t.Fatal("unknown daemon field crossed the projection")
	}
}

func TestProjectionOmitsMissingSize(t *testing.T) {
	volume := VolumeData(VolumeSummary{Name: "data", Driver: "local"})
	if _, ok := volume["size"]; ok {
		t.Fatal("missing volume size must be omitted, never zero")
	}
	image := ImageData(ImageSummary{ID: "sha256:x"})
	if _, ok := image["size"]; ok {
		t.Fatal("missing image size must be omitted")
	}
}

func TestReclaimableNeverClaimsDuration(t *testing.T) {
	data := ReclaimableData(&Reclaimable{ObservationStable: true, UnusedImages: []string{"a"}})
	for key := range data {
		switch key {
		case "observation_stable", "unused_image_ids", "unused_image_unique_bytes", "unused_volume_names", "unused_volume_bytes", "stopped_container_ids", "dangling_image_ids", "criteria", "advisory":
		default:
			t.Fatalf("reclaimable projection carries unmaintained key %s", key)
		}
	}
	for _, banned := range []string{"unused_since", "unused_duration", "last_use"} {
		if _, ok := data[banned]; ok {
			t.Fatalf("reclaimable must not claim %s", banned)
		}
	}
}

func TestLogPageProjection(t *testing.T) {
	when := time.Now().UTC()
	data := LogPageData(&LogPage{Records: []LogRecord{{Stream: "stdout", Timestamp: &when, Message: "hello"}}, Truncated: true, Skipped: 2})
	if _, ok := data["entries"].([]LogRecord); !ok {
		t.Fatal("records must survive projection")
	}
	if data["truncated"] != true || data["skipped_frames"] != 2 {
		t.Fatal("truncation evidence lost")
	}
}
