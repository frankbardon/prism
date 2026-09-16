package spec

import (
	"encoding/json"
	"testing"
)

func TestPrismSortDirection(t *testing.T) {
	cases := []struct {
		in    string
		desc  bool
		valid bool
	}{
		{"", false, true},
		{"ascending", false, true},
		{"asc", false, true},
		{"descending", true, true},
		{"desc", true, true},
		{"DESC", false, false},
		{"reverse", false, false},
	}
	for _, tc := range cases {
		if got := SortDirectionDescending(tc.in); got != tc.desc {
			t.Errorf("SortDirectionDescending(%q) = %v, want %v", tc.in, got, tc.desc)
		}
		if got := SortDirectionValid(tc.in); got != tc.valid {
			t.Errorf("SortDirectionValid(%q) = %v, want %v", tc.in, got, tc.valid)
		}
	}
}

func TestPrismResolveOrder(t *testing.T) {
	t.Run("unbound returns nil", func(t *testing.T) {
		if got := ResolveOrder(&Encoding{}); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
		if got := ResolveOrder(nil); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		enc := &Encoding{Order: &OrderChannel{
			Single: &OrderChannelEntry{Field: "rank", Sort: "descending"},
		}}
		got := ResolveOrder(enc)
		if len(got) != 1 || got[0].Field != "rank" || !got[0].Descending {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("array keeps spec order", func(t *testing.T) {
		enc := &Encoding{Order: &OrderChannel{Multi: []OrderChannelEntry{
			{Field: "tier"},
			{Field: "rev", Sort: "desc"},
		}}}
		got := ResolveOrder(enc)
		if len(got) != 2 {
			t.Fatalf("got %d keys, want 2", len(got))
		}
		if got[0].Field != "tier" || got[0].Descending {
			t.Fatalf("key 0 = %+v", got[0])
		}
		if got[1].Field != "rev" || !got[1].Descending {
			t.Fatalf("key 1 = %+v", got[1])
		}
	})

	t.Run("fieldless entries drop out", func(t *testing.T) {
		enc := &Encoding{Order: &OrderChannel{
			Single: &OrderChannelEntry{Type: "quantitative"},
		}}
		if got := ResolveOrder(enc); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})

	t.Run("decoded from JSON, both forms", func(t *testing.T) {
		for _, raw := range []string{
			`{"order": {"field": "rank", "sort": "descending"}}`,
			`{"order": [{"field": "rank", "sort": "descending"}]}`,
		} {
			var enc Encoding
			if err := json.Unmarshal([]byte(raw), &enc); err != nil {
				t.Fatalf("decode %s: %v", raw, err)
			}
			got := ResolveOrder(&enc)
			if len(got) != 1 || got[0].Field != "rank" || !got[0].Descending {
				t.Fatalf("%s resolved to %+v", raw, got)
			}
		}
	})
}
