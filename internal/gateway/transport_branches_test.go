package gateway

import (
	"net/http/httptest"
	"testing"
)

func TestMeasuredResponseFlushWritesImplicitStatus(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	measured := &measuredResponse{ResponseWriter: rec}
	measured.Flush()
	if rec.Code != 200 {
		t.Fatalf("implicit flush status = %d, want 200", rec.Code)
	}
	if measured.status != 200 {
		t.Fatalf("flush must record the implicit status, got %d", measured.status)
	}
	// A second flush must not rewrite the status header.
	measured.Flush()
	if rec.Code != 200 {
		t.Fatalf("repeated flush changed the status: %d", rec.Code)
	}
}

func TestMeasuredResponseWriteHeaderWinsOnce(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	measured := &measuredResponse{ResponseWriter: rec}
	measured.WriteHeader(201)
	if measured.status != 201 {
		t.Fatalf("explicit status lost: %d", measured.status)
	}
	// The wrapper must never rewrite an already-written status.
	measured.WriteHeader(500)
	if measured.status != 201 {
		t.Fatalf("status rewritten: %d", measured.status)
	}
}

func TestMeasuredResponseWriteImpliesSuccess(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	measured := &measuredResponse{ResponseWriter: rec}
	if _, e := measured.Write([]byte("body")); e != nil {
		t.Fatal(e)
	}
	if measured.status != 200 || rec.Code != 200 {
		t.Fatalf("implicit write status = %d/%d", measured.status, rec.Code)
	}
}

func TestMCPEnvelopesAcceptArrayObjectAndRejectGarbage(t *testing.T) {
	t.Parallel()

	array, e := mcpEnvelopes([]byte(`[{"id":1,"method":"tools/call","params":{"name":"get_os_info"}},{"id":2,"method":"tools/list"}]`))
	if e != nil || len(array) != 2 || array[0].Method != "tools/call" || array[1].Params.Name != "" {
		t.Fatalf("array envelopes lost: %v %v", array, e)
	}
	single, e := mcpEnvelopes([]byte(`{"id":7,"method":"tools/call","params":{"name":"read_config"}}`))
	if e != nil || len(single) != 1 || single[0].Params.Name != "read_config" {
		t.Fatalf("single envelope lost: %v %v", single, e)
	}
	if _, e := mcpEnvelopes([]byte("{not json")); e == nil {
		t.Fatal("malformed object accepted")
	}
	if _, e := mcpEnvelopes([]byte("[not json")); e == nil {
		t.Fatal("malformed array accepted")
	}
	if _, e := mcpEnvelopes(nil); e == nil {
		t.Fatal("empty payload accepted")
	}
}
