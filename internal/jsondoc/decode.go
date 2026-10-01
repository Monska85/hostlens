package jsondoc

import (
	"encoding/json"
	"errors"
	"io"
)

// Decode reads exactly one JSON document within maxBytes. It also counts
// trailing whitespace, so a short value cannot hide an oversized body.
func Decode(reader io.Reader, maxBytes int64, out any) error {
	return decode(reader, maxBytes, out, false)
}

// DecodeStrict applies the same bound and rejects unknown object fields.
func DecodeStrict(reader io.Reader, maxBytes int64, out any) error {
	return decode(reader, maxBytes, out, true)
}

func decode(reader io.Reader, maxBytes int64, out any, strict bool) error {
	limited := &io.LimitedReader{R: reader, N: maxBytes + 1}
	decoder := json.NewDecoder(limited)
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("one valid JSON document required")
	}
	if limited.N == 0 {
		return errors.New("JSON document exceeds size limit")
	}
	return nil
}
