package nodes

import (
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// crosstabTestSchema is the native input schema the crosstab node
// derives its output shape from.
func crosstabTestSchema() *table.Schema {
	return &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "order_date", Type: table.FieldTypeDate},
		{Name: "d", Type: table.FieldTypeDate},
		{Name: "revenue", Type: table.FieldTypeF64},
	}}
}

// TestNewCrosstabDateGrouperSchema verifies a date grouper produces a
// string (categorical) bucket-key column in the output schema while a
// category grouper preserves its source field type, and the cell column
// carries the aggregate output type.
func TestNewCrosstabDateGrouperSchema(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "order_date", Type: "date", Period: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "rev"},
	}
	n, err := NewCrosstab("ct", "src", "sales.pulse", afero.NewMemMapFs(), body)
	if err != nil {
		t.Fatalf("NewCrosstab: %v", err)
	}
	out, err := n.Schema([]*table.Schema{crosstabTestSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	byName := map[string]table.FieldType{}
	for _, f := range out.Fields {
		byName[f.Name] = f.Type
	}
	if got := byName["region"]; got != table.FieldTypeCategoricalU8 {
		t.Errorf("region output type = %v, want CategoricalU8 (source type preserved)", got)
	}
	if got := byName["order_date"]; got != table.FieldTypeCategoricalU32 {
		t.Errorf("date grouper output type = %v, want CategoricalU32 (string bucket key)", got)
	}
	if got := byName["rev"]; got != table.FieldTypeF64 {
		t.Errorf("cell output type = %v, want F64", got)
	}
}

// TestNewCrosstabDateDefaultPeriod verifies an empty date period is
// accepted (defaults to month at execute time).
func TestNewCrosstabDateDefaultPeriod(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "d", Type: "date"}},
		Columns: []spec.CrosstabGroup{{Field: "region"}},
		Cell:    spec.CrosstabCell{Aggregate: "count"},
	}
	if _, err := NewCrosstab("ct", "src", "c.pulse", afero.NewMemMapFs(), body); err != nil {
		t.Fatalf("NewCrosstab default period: %v", err)
	}
}

// TestNewCrosstabBadPeriod verifies an invalid date period is rejected
// at construction with PRISM_SPEC_032.
func TestNewCrosstabBadPeriod(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "d", Type: "date", Period: "fortnight"}},
		Columns: []spec.CrosstabGroup{{Field: "region"}},
		Cell:    spec.CrosstabCell{Aggregate: "count"},
	}
	if _, err := NewCrosstab("ct", "src", "c.pulse", afero.NewMemMapFs(), body); err == nil {
		t.Fatal("expected error for invalid date period, got nil")
	}
}

// TestNewCrosstabUnknownAggregate verifies an unsupported cell aggregate
// alias is rejected at construction.
func TestNewCrosstabUnknownAggregate(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "order_date", Type: "date"}},
		Cell:    spec.CrosstabCell{Aggregate: "bogus", Field: "revenue"},
	}
	if _, err := NewCrosstab("ct", "src", "s.pulse", afero.NewMemMapFs(), body); err == nil {
		t.Fatal("expected error for unknown cell aggregate, got nil")
	}
}

// TestNewCrosstabCountNeedsNoField verifies count without a field is
// accepted (count(*)), while a non-count aggregate without a field is
// rejected.
func TestNewCrosstabCellFieldRequirement(t *testing.T) {
	ok := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "d", Type: "date"}},
		Cell:    spec.CrosstabCell{Aggregate: "count"},
	}
	if _, err := NewCrosstab("ct", "src", "s.pulse", afero.NewMemMapFs(), ok); err != nil {
		t.Fatalf("count(*) should be allowed: %v", err)
	}
	bad := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "d", Type: "date"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum"},
	}
	if _, err := NewCrosstab("ct", "src", "s.pulse", afero.NewMemMapFs(), bad); err == nil {
		t.Fatal("expected error for sum aggregate without a field, got nil")
	}
}
