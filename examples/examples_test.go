package examples

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestListReturnsValidStemsSortedNoInvalid(t *testing.T) {
	got := List()
	if len(got) == 0 {
		t.Fatal("List() returned no specs")
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("List() is not sorted: %v", got)
	}
	for _, name := range got {
		if name == invalidDir || strings.HasPrefix(name, invalidDir+"/") {
			t.Errorf("List() leaked an invalid spec: %q", name)
		}
	}
	// Spot-check a known top-level stem and a known subdir stem.
	if !slices.Contains(got, "bar_basic") {
		t.Errorf("List() missing bar_basic; got %v", got)
	}
	if !slices.Contains(got, "scales/log") {
		t.Errorf("List() missing scales/log; got %v", got)
	}
}

func TestInvalidAndAll(t *testing.T) {
	inv := Invalid()
	if len(inv) == 0 {
		t.Fatal("Invalid() returned nothing; expected designed-to-fail specs")
	}
	for _, name := range inv {
		if !strings.HasPrefix(name, invalidDir+"/") {
			t.Errorf("Invalid() returned a non-invalid stem: %q", name)
		}
	}
	if !slices.Contains(inv, "invalid/theta_on_bar") {
		t.Errorf("Invalid() missing invalid/theta_on_bar; got %v", inv)
	}

	all := All()
	if len(all) != len(List())+len(inv) {
		t.Errorf("All() = %d, want %d (valid) + %d (invalid)", len(all), len(List()), len(inv))
	}
	if !sort.StringsAreSorted(all) {
		t.Errorf("All() is not sorted: %v", all)
	}
}

func TestGet(t *testing.T) {
	body, ok := Get("bar_basic")
	if !ok {
		t.Fatal("Get(bar_basic) not found")
	}
	if len(body) == 0 || !strings.Contains(string(body), "\"mark\"") {
		t.Errorf("Get(bar_basic) returned unexpected body: %q", string(body))
	}

	// Subdir stem.
	if _, ok := Get("scales/log"); !ok {
		t.Error("Get(scales/log) not found")
	}
	// Invalid specs are reachable by stem.
	if _, ok := Get("invalid/theta_on_bar"); !ok {
		t.Error("Get(invalid/theta_on_bar) not found")
	}
	// Missing stem.
	if _, ok := Get("does_not_exist"); ok {
		t.Error("Get(does_not_exist) reported found")
	}
	// A bare ".json" suffix in the name should not double-append.
	if _, ok := Get("bar_basic.json"); ok {
		t.Error("Get should take a stem, not a filename with .json")
	}
}

func TestSearch(t *testing.T) {
	// Title-based match: bar_basic has title "Brand score (basic bar)".
	hits := Search("brand score", 10)
	if len(hits) == 0 {
		t.Fatal("Search(brand score) returned nothing")
	}
	found := false
	for _, h := range hits {
		if h.Name == "bar_basic" {
			found = true
			if h.Summary != "Brand score (basic bar)" {
				t.Errorf("Summary = %q, want title", h.Summary)
			}
			if !strings.Contains(h.Spec, "\"mark\"") {
				t.Errorf("Spec not raw JSON: %q", h.Spec)
			}
		}
	}
	if !found {
		t.Errorf("Search(brand score) missing bar_basic; got %v", names(hits))
	}

	// Results sorted by name.
	stems := names(hits)
	if !sort.StringsAreSorted(stems) {
		t.Errorf("Search results not sorted by name: %v", stems)
	}

	// Limit is honored.
	if got := Search("", 3); len(got) != 3 {
		t.Errorf("Search(\"\", 3) returned %d results, want 3", len(got))
	}

	// Invalid specs never surface in Search.
	for _, h := range Search("", 0) {
		if strings.HasPrefix(h.Name, invalidDir+"/") {
			t.Errorf("Search leaked invalid spec: %q", h.Name)
		}
	}

	// Stem-based match (no such title text, only the filename).
	if got := Search("scales/log", 5); len(got) != 1 || got[0].Name != "scales/log" {
		t.Errorf("Search(scales/log) = %v, want exactly scales/log", names(got))
	}
}

func names(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}
