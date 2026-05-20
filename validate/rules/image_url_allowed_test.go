package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismImageURLAcceptsDataURL(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark: &spec.Mark{Def: &spec.MarkDef{
			Type: "image",
			URL:  "data:image/png;base64,iVBORw0KGgo=",
		}},
	}
	errs := ImageURLAllowed{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("data: URL must be allowed, got: %+v", errs)
	}
}

func TestPrismImageURLAcceptsRelativePath(t *testing.T) {
	cases := []string{"logo.png", "./assets/foo.svg", "assets/foo.svg", "a/b/c.jpg"}
	for _, url := range cases {
		s := &spec.Spec{
			Schema: "urn:prism:schema:v1:spec",
			Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "image", URL: url}},
		}
		errs := ImageURLAllowed{}.Check(s, validate.EmptyLookup{})
		if len(errs) != 0 {
			t.Errorf("relative path %q must be allowed, got: %+v", url, errs)
		}
	}
}

func TestPrismImageURLRejectsRemote(t *testing.T) {
	cases := []string{
		"https://evil.example/x.png",
		"http://example.com/foo.svg",
		"ftp://files/bar.jpg",
		"file:///etc/passwd",
		"gs://bucket/key.png",
		"s3://bucket/key.png",
		"/absolute/path.png",
	}
	for _, url := range cases {
		s := &spec.Spec{
			Schema: "urn:prism:schema:v1:spec",
			Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "image", URL: url}},
		}
		errs := ImageURLAllowed{}.Check(s, validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_016" {
			t.Errorf("URL %q: expected PRISM_SPEC_016, got: %+v", url, errs)
		}
	}
}

func TestPrismImageURLIgnoresNonImageMark(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	errs := ImageURLAllowed{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Errorf("non-image mark should be ignored, got: %+v", errs)
	}
}
