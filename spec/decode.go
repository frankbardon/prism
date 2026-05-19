package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Decode reads a Prism spec from r with strict decoding enabled
// (DisallowUnknownFields). All callers in the validator and CLI route
// through here so the strictness invariant lives in one place.
func Decode(r io.Reader) (*Spec, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var s Spec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}
	return &s, nil
}

// DecodeBytes is the byte-slice convenience wrapper around Decode.
func DecodeBytes(data []byte) (*Spec, error) {
	return Decode(bytes.NewReader(data))
}
