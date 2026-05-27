package prism

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

// Patch is an RFC 6902 JSON Patch — an ordered list of operations to
// transform one spec into another. Use ApplyPatch to apply a patch
// to a spec; DiffSpecs to compute one between two specs.
//
// Atomic semantics: ApplyPatch either succeeds with every operation
// applied, or fails with no state change. A failing operation's
// index is returned in the error envelope.
type Patch []PatchOp

// PatchOp is one RFC 6902 operation.
//
//	Op:    "add" | "remove" | "replace" | "move" | "copy" | "test"
//	Path:  JSON Pointer (RFC 6901) targeting the value to operate on.
//	Value: required for add / replace / test.
//	From:  required for move / copy (the source pointer).
type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
	From  string `json:"from,omitempty"`
}

// ApplyPatch returns a new *spec.Spec with the patch's operations
// applied. The input spec is not mutated. On any operation failure
// (path miss, type mismatch, schema violation post-apply), the
// returned spec is nil and the error envelope carries
// PRISM_SPEC_PATCH_001 with the failing operation index in Details.
func ApplyPatch(s *spec.Spec, p Patch) (*spec.Spec, error) {
	if s == nil {
		return nil, fmt.Errorf("prism.ApplyPatch: nil spec")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, prismerrors.New("PRISM_SPEC_PATCH_001",
			fmt.Sprintf("encode current spec: %v", err),
			map[string]any{"OpIndex": -1})
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, prismerrors.New("PRISM_SPEC_PATCH_001",
			fmt.Sprintf("decode current spec for patching: %v", err),
			map[string]any{"OpIndex": -1})
	}
	for i, op := range p {
		next, err := applyOp(tree, op)
		if err != nil {
			return nil, prismerrors.New("PRISM_SPEC_PATCH_001",
				fmt.Sprintf("operation %d (%s %q): %v", i, op.Op, op.Path, err),
				map[string]any{"OpIndex": i, "Op": op.Op, "Path": op.Path})
		}
		tree = next
	}
	body, err := json.Marshal(tree)
	if err != nil {
		return nil, prismerrors.New("PRISM_SPEC_PATCH_001",
			fmt.Sprintf("re-encode patched spec: %v", err),
			map[string]any{"OpIndex": -1})
	}
	out, err := spec.DecodeBytes(body)
	if err != nil {
		return nil, prismerrors.New("PRISM_SPEC_PATCH_001",
			fmt.Sprintf("patched spec failed to decode: %v", err),
			map[string]any{"OpIndex": -1})
	}
	return out, nil
}

// DiffSpecs returns a Patch that, when applied to before, produces
// after. The result is a sequence of `replace` / `add` / `remove`
// operations expressed against the JSON form of the specs — not
// necessarily the minimal patch, but a correct one. Useful for
// callers that want to think in full specs and transmit minimal
// updates.
func DiffSpecs(before, after *spec.Spec) (Patch, error) {
	beforeRaw, err := json.Marshal(before)
	if err != nil {
		return nil, fmt.Errorf("prism.DiffSpecs: encode before: %w", err)
	}
	afterRaw, err := json.Marshal(after)
	if err != nil {
		return nil, fmt.Errorf("prism.DiffSpecs: encode after: %w", err)
	}
	var beforeTree, afterTree any
	if err := json.Unmarshal(beforeRaw, &beforeTree); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(afterRaw, &afterTree); err != nil {
		return nil, err
	}
	var ops Patch
	diff("", beforeTree, afterTree, &ops)
	return ops, nil
}

// applyOp returns the tree with op applied, or an error. The
// returned tree shares structure with the input when the operation
// doesn't touch a subtree — we only deep-copy the path that mutates.
func applyOp(tree any, op PatchOp) (any, error) {
	switch op.Op {
	case "add":
		return setAt(tree, op.Path, op.Value, true)
	case "replace":
		return setAt(tree, op.Path, op.Value, false)
	case "remove":
		return removeAt(tree, op.Path)
	case "test":
		got, err := getAt(tree, op.Path)
		if err != nil {
			return nil, err
		}
		if !equalJSON(got, op.Value) {
			return nil, fmt.Errorf("test failed: path %q current value differs from expected", op.Path)
		}
		return tree, nil
	case "move":
		val, err := getAt(tree, op.From)
		if err != nil {
			return nil, fmt.Errorf("move from %q: %v", op.From, err)
		}
		t, err := removeAt(tree, op.From)
		if err != nil {
			return nil, fmt.Errorf("move from %q: %v", op.From, err)
		}
		return setAt(t, op.Path, val, true)
	case "copy":
		val, err := getAt(tree, op.From)
		if err != nil {
			return nil, fmt.Errorf("copy from %q: %v", op.From, err)
		}
		return setAt(tree, op.Path, deepCopy(val), true)
	}
	return nil, fmt.Errorf("unknown op %q (want add|remove|replace|move|copy|test)", op.Op)
}

// parsePointer decomposes an RFC 6901 JSON pointer into segments.
// "" → []; "/" → [""]; "/foo/bar" → ["foo","bar"]. Tokens decode the
// "~1" → "/" and "~0" → "~" escapes.
func parsePointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("pointer must start with /, got %q", p)
	}
	parts := strings.Split(p[1:], "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

// getAt resolves the value at pointer p in tree.
func getAt(tree any, p string) (any, error) {
	parts, err := parsePointer(p)
	if err != nil {
		return nil, err
	}
	cur := tree
	for _, part := range parts {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path %q: key %q missing", p, part)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("path %q: bad array index %q", p, part)
			}
			cur = v[idx]
		default:
			return nil, fmt.Errorf("path %q: cannot descend through %T", p, cur)
		}
	}
	return cur, nil
}

// setAt returns a copy of tree with value placed at pointer p. When
// add=true and the parent is an array, "-" appends; an integer index
// inserts (shifts the tail). When add=false the operation is a
// replace — the parent must already have a value at that slot.
func setAt(tree any, p string, value any, add bool) (any, error) {
	parts, err := parsePointer(p)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		// Replacing the root.
		return deepCopy(value), nil
	}
	// Walk to the parent.
	clone := deepCopy(tree)
	parent, err := walkParent(clone, parts)
	if err != nil {
		return nil, err
	}
	last := parts[len(parts)-1]
	switch p := parent.(type) {
	case map[string]any:
		if !add {
			if _, ok := p[last]; !ok {
				return nil, fmt.Errorf("path %q: key %q missing (replace requires presence)", parts, last)
			}
		}
		p[last] = deepCopy(value)
	case *[]any:
		arr := *p
		if last == "-" {
			arr = append(arr, deepCopy(value))
			*p = arr
			return clone, nil
		}
		idx, err := strconv.Atoi(last)
		if err != nil || idx < 0 {
			return nil, fmt.Errorf("path %q: bad array index %q", parts, last)
		}
		if add {
			if idx > len(arr) {
				return nil, fmt.Errorf("path %q: index %d out of range (len %d)", parts, idx, len(arr))
			}
			arr = append(arr, nil)
			copy(arr[idx+1:], arr[idx:])
			arr[idx] = deepCopy(value)
			*p = arr
		} else {
			if idx >= len(arr) {
				return nil, fmt.Errorf("path %q: index %d out of range (len %d)", parts, idx, len(arr))
			}
			arr[idx] = deepCopy(value)
		}
	default:
		return nil, fmt.Errorf("path %q: parent is %T (want object or array)", parts, parent)
	}
	return clone, nil
}

// removeAt returns a copy of tree with the value at p deleted.
func removeAt(tree any, p string) (any, error) {
	parts, err := parsePointer(p)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("path %q: cannot remove root", p)
	}
	clone := deepCopy(tree)
	parent, err := walkParent(clone, parts)
	if err != nil {
		return nil, err
	}
	last := parts[len(parts)-1]
	switch v := parent.(type) {
	case map[string]any:
		if _, ok := v[last]; !ok {
			return nil, fmt.Errorf("path %q: key %q missing", p, last)
		}
		delete(v, last)
	case *[]any:
		arr := *v
		idx, err := strconv.Atoi(last)
		if err != nil || idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("path %q: bad array index %q", p, last)
		}
		arr = append(arr[:idx], arr[idx+1:]...)
		*v = arr
	default:
		return nil, fmt.Errorf("path %q: parent is %T", p, parent)
	}
	return clone, nil
}

// walkParent descends through every segment except the last,
// returning the parent container. Arrays are returned as *[]any so
// callers can mutate via the pointer.
func walkParent(tree any, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	if len(parts) == 1 {
		return containerHandle(tree), nil
	}
	parent, err := walkParent(tree, parts[:len(parts)-1])
	if err != nil {
		return nil, err
	}
	switch v := parent.(type) {
	case map[string]any:
		key := parts[len(parts)-2]
		child, ok := v[key]
		if !ok {
			return nil, fmt.Errorf("path %q: key %q missing", parts, key)
		}
		return containerHandle(child), nil
	case *[]any:
		idxStr := parts[len(parts)-2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx < 0 || idx >= len(*v) {
			return nil, fmt.Errorf("path %q: bad index %q", parts, idxStr)
		}
		return containerHandle((*v)[idx]), nil
	default:
		return nil, fmt.Errorf("path %q: cannot descend through %T", parts, parent)
	}
}

// containerHandle normalises map / slice returns so arrays come back
// as *[]any (mutable) and maps stay as map[string]any. Other values
// (root non-container) pass through verbatim.
func containerHandle(v any) any {
	switch x := v.(type) {
	case []any:
		return &x
	default:
		return v
	}
}

// deepCopy clones a tree of JSON-typed values (object/array/scalar).
func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = deepCopy(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopy(vv)
		}
		return out
	default:
		return v
	}
}

// equalJSON compares two JSON-typed values for deep equality.
func equalJSON(a, b any) bool {
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	return string(ra) == string(rb)
}

// diff emits ops into *ops that transform b → a. Uses the simplest
// correct strategy: when types or values differ at a level, emit a
// replace at that path. Walks objects recursively to keep ops scoped.
func diff(path string, before, after any, ops *Patch) {
	if equalJSON(before, after) {
		return
	}
	// If either side isn't an object, emit a top-level replace.
	bm, bMap := before.(map[string]any)
	am, aMap := after.(map[string]any)
	if !bMap || !aMap {
		*ops = append(*ops, PatchOp{Op: "replace", Path: pathOrRoot(path), Value: after})
		return
	}
	// Both maps — walk keys.
	for k, av := range am {
		child := path + "/" + escapeToken(k)
		bv, hasB := bm[k]
		if !hasB {
			*ops = append(*ops, PatchOp{Op: "add", Path: child, Value: av})
			continue
		}
		diff(child, bv, av, ops)
	}
	for k := range bm {
		if _, hasA := am[k]; !hasA {
			*ops = append(*ops, PatchOp{Op: "remove", Path: path + "/" + escapeToken(k)})
		}
	}
}

func pathOrRoot(p string) string {
	if p == "" {
		return ""
	}
	return p
}

func escapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
