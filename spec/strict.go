package spec

import (
	"bytes"
	"encoding/json"
	"strings"
)

// strictUnmarshal decodes data into v with unknown JSON keys rejected.
//
// It exists because Go does NOT propagate a json.Decoder's
// DisallowUnknownFields setting into a custom UnmarshalJSON: a
// hand-written decoder receives raw bytes and any json.Unmarshal it
// calls is unconditionally lenient. spec.Decode sets
// DisallowUnknownFields on the outer decoder, so every hand-written
// decoder in this package has to re-arm strictness itself or the
// promise in the package doc ("Decoding is strict: unknown fields
// fail") holds only for the structs that have no custom decoder.
//
// internal/gates/spec_strict_decode_test.go gates that: it enumerates
// every UnmarshalJSON method in this package and drives an unknown key
// through each one.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// isUnknownFieldError reports whether err came from
// DisallowUnknownFields rather than from a type or syntax mismatch.
// encoding/json reports it as a plain *errors.errorString, so the text
// is the only discriminator available. Used by the scalar-or-object
// union decoders, which otherwise swallow a decode failure and fall
// through to their own "expected X or Y" message — unhelpful when the
// real problem is one misspelled key inside an object that is
// otherwise the right shape.
func isUnknownFieldError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown field")
}
