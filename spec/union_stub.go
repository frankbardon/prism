package spec

import "errors"

// errMarkUnionStub is returned by the placeholder Mark/Transform unmarshal
// methods until T01.14 wires up the real discriminated-union logic.
var errMarkUnionStub = errors.New("spec: discriminated unions wired in T01.14")

// UnmarshalJSON is a temporary stub; T01.14 replaces it with the real
// discriminated-union decoder. Defined here so the spec package compiles
// after T01.13 alone.
func (m *Mark) UnmarshalJSON(_ []byte) error { return errMarkUnionStub }

// UnmarshalJSON is a temporary stub; T01.14 replaces it.
func (t *Transform) UnmarshalJSON(_ []byte) error { return errMarkUnionStub }
