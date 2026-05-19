package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestPrismSchemaFilesEmbedded verifies the //go:embed FS matches the
// on-disk schema directory byte-for-byte. Adding a new schema file
// without updating embed directives or schema_files lists will fail here.
func TestPrismSchemaFilesEmbedded(t *testing.T) {
	embedded, err := V1Schemas()
	if err != nil {
		t.Fatalf("V1Schemas: %v", err)
	}

	diskRoot := filepath.Join("v1")
	diskFiles := map[string][]byte{}
	err = filepath.Walk(diskRoot, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(filepath.ToSlash(p), "/_meta/") {
			return nil
		}
		if !strings.HasSuffix(p, ".schema.json") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		base := strings.TrimSuffix(filepath.Base(p), ".schema.json")
		diskFiles[base] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk disk: %v", err)
	}

	if len(embedded) != len(diskFiles) {
		t.Fatalf("embedded count %d != disk count %d (embedded=%v disk=%v)",
			len(embedded), len(diskFiles), keysOf(embedded), keysOf(diskFiles))
	}
	for name, want := range diskFiles {
		got, ok := embedded[name]
		if !ok {
			t.Errorf("disk has %s but embed does not", name)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("schema %s: embed bytes differ from disk", name)
		}
	}
}

// TestPrismSchemaURNsCanonical ensures every schema's $id is a URN of the
// form urn:prism:schema:v1:<basename>. No URLs, no version drift, no typos.
func TestPrismSchemaURNsCanonical(t *testing.T) {
	urnPattern := regexp.MustCompile(`^urn:prism:schema:v1:[a-z][a-z0-9_]*$`)
	schemas, err := V1Schemas()
	if err != nil {
		t.Fatalf("V1Schemas: %v", err)
	}
	for name, raw := range schemas {
		var head struct {
			ID string `json:"$id"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if !urnPattern.MatchString(head.ID) {
			t.Errorf("%s: $id %q does not match canonical URN pattern", name, head.ID)
		}
		want := URNPrefix + name
		if head.ID != want {
			t.Errorf("%s: $id %q != expected %q", name, head.ID, want)
		}
	}
}

// TestPrismRelativeRefsResolveLocally walks every $ref string in every
// embedded schema and confirms relative refs (filename#/path) resolve to a
// file present in the bundle and that any URN refs use the v1 namespace.
func TestPrismRelativeRefsResolveLocally(t *testing.T) {
	schemas, err := V1Schemas()
	if err != nil {
		t.Fatalf("V1Schemas: %v", err)
	}
	known := map[string]bool{}
	for name := range schemas {
		known[name+".schema.json"] = true
	}

	for name, raw := range schemas {
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		walkRefs(doc, func(ref string) {
			switch {
			case strings.HasPrefix(ref, "urn:prism:schema:v1:"):
				// URN refs are accepted; we only verify they target known schemas.
				target := strings.TrimPrefix(ref, "urn:prism:schema:v1:")
				if !known[target+".schema.json"] {
					t.Errorf("%s: URN $ref %q targets unknown schema", name, ref)
				}
			case strings.HasPrefix(ref, "#"):
				// Local intra-file ref; pointer correctness is the validator's job.
			default:
				// Expected shape: <filename>.schema.json#/...
				hash := strings.IndexByte(ref, '#')
				file := ref
				if hash >= 0 {
					file = ref[:hash]
				}
				if file == "" {
					t.Errorf("%s: $ref %q missing filename", name, ref)
					return
				}
				if !known[file] {
					t.Errorf("%s: $ref %q points to file not in embedded bundle (known=%v)", name, ref, sortedKeys(known))
				}
			}
		})
	}
}

func walkRefs(node any, visit func(string)) {
	switch v := node.(type) {
	case map[string]any:
		if r, ok := v["$ref"].(string); ok {
			visit(r)
		}
		for _, child := range v {
			walkRefs(child, visit)
		}
	case []any:
		for _, child := range v {
			walkRefs(child, visit)
		}
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
