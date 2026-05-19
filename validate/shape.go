// Package validate runs the two-stage Prism spec validator:
//
//  1. ShapeValidator runs the JSON Schema bundle from schema/embed.go,
//     which catches structural errors (unknown fields, missing required,
//     wrong types, oneOf misses).
//  2. SemanticValidator (semantic.go) runs Go-side rules that need
//     schema-aware reasoning (PRISM_SPEC_001..009).
//
// The two stages are kept separate so a spec that fails shape never reaches
// the semantic stage; many semantic rules assume the spec is structurally
// well-formed.
package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/frankbardon/prism/schema"
)

// errPrinter renders ErrorKind.LocalizedString messages.
var errPrinter = message.NewPrinter(language.English)

// ShapeValidator wraps a compiled JSON Schema for the Prism spec entry
// point. Construction is cheap once; callers should reuse a validator
// instance across many Validate calls.
type ShapeValidator struct {
	compiled *jsonschema.Schema
}

// NewShapeValidator builds a ShapeValidator backed by the embedded v1
// schema bundle. Every schema file is registered under both its filename
// (so relative $refs like "data.schema.json#/$defs/data" resolve) and
// its URN $id (so urn:prism:schema:v1:spec self-refs in composition.json
// resolve).
func NewShapeValidator() (*ShapeValidator, error) {
	bundle, err := schema.V1Schemas()
	if err != nil {
		return nil, fmt.Errorf("load embedded schemas: %w", err)
	}

	c := jsonschema.NewCompiler()
	for name, raw := range bundle {
		var doc any
		if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		// Rewrite every relative "name.schema.json#/..." $ref to its URN
		// form so refs resolve regardless of which base URL the compiler
		// uses for the document. Each schema's $id is already a URN, so
		// the URN namespace is the only consistent resolution root.
		// See validate/RULES.md for the design rationale.
		rewriteRelativeRefs(doc)
		urn := schema.URNFor(name)
		if err := c.AddResource(urn, doc); err != nil {
			return nil, fmt.Errorf("add resource %s: %w", urn, err)
		}
	}

	compiled, err := c.Compile(schema.URNFor("spec"))
	if err != nil {
		return nil, fmt.Errorf("compile spec schema: %w", err)
	}
	return &ShapeValidator{compiled: compiled}, nil
}

// Validate runs the compiled JSON Schema against spec (which must be a
// generic JSON value: map[string]any / []any / scalars — i.e. the output
// of json.Unmarshal into interface{}). Returns a flat slice of shape
// errors; an empty slice means the spec is structurally valid.
func (v *ShapeValidator) Validate(spec any) []ShapeError {
	if err := v.compiled.Validate(spec); err != nil {
		return flattenShapeErrors(err)
	}
	return nil
}

// ShapeError is the validator-agnostic representation of one shape failure.
type ShapeError struct {
	// InstanceLocation is the JSON pointer to the offending instance node.
	InstanceLocation string
	// KeywordLocation is the JSON pointer to the schema keyword that failed.
	KeywordLocation string
	// Message is the human-readable failure description.
	Message string
}

// flattenShapeErrors walks the recursive *ValidationError tree and returns
// the leaf failures in stable, depth-first order.
func flattenShapeErrors(err error) []ShapeError {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []ShapeError{{Message: err.Error()}}
	}
	var out []ShapeError
	var walk func(*jsonschema.ValidationError)
	walk = func(node *jsonschema.ValidationError) {
		if len(node.Causes) == 0 {
			kw := ""
			if node.ErrorKind != nil {
				kw = jsonPointer(node.ErrorKind.KeywordPath())
			}
			msg := ""
			if node.ErrorKind != nil {
				msg = node.ErrorKind.LocalizedString(errPrinter)
			}
			out = append(out, ShapeError{
				InstanceLocation: jsonPointer(node.InstanceLocation),
				KeywordLocation:  kw,
				Message:          msg,
			})
			return
		}
		for _, c := range node.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

// rewriteRelativeRefs walks doc and converts every "$ref" of the form
// "<name>.schema.json[#...]" into "urn:prism:schema:v1:<name>[#...]".
// Intra-file refs ("#/$defs/x") and refs already in URN form are left
// untouched.
func rewriteRelativeRefs(node any) {
	switch v := node.(type) {
	case map[string]any:
		if r, ok := v["$ref"].(string); ok {
			if rewritten, changed := rewriteRef(r); changed {
				v["$ref"] = rewritten
			}
		}
		for _, child := range v {
			rewriteRelativeRefs(child)
		}
	case []any:
		for _, child := range v {
			rewriteRelativeRefs(child)
		}
	}
}

func rewriteRef(ref string) (string, bool) {
	if strings.HasPrefix(ref, "urn:") || strings.HasPrefix(ref, "#") {
		return ref, false
	}
	// Expected: "<name>.schema.json[#/path]"
	file := ref
	frag := ""
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		file = ref[:i]
		frag = ref[i:]
	}
	if !strings.HasSuffix(file, ".schema.json") {
		return ref, false
	}
	name := strings.TrimSuffix(file, ".schema.json")
	return schema.URNFor(name) + frag, true
}

func jsonPointer(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	var b bytes.Buffer
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(p)
	}
	return b.String()
}
