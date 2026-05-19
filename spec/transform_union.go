package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalJSON emits the JSON payload of whichever transform variant is
// populated.
func (t Transform) MarshalJSON() ([]byte, error) {
	switch {
	case t.Filter != nil:
		return json.Marshal(t.Filter)
	case t.Calculate != nil:
		return json.Marshal(t.Calculate)
	case t.Aggregate != nil:
		return json.Marshal(t.Aggregate)
	case t.Bin != nil:
		return json.Marshal(t.Bin)
	case t.Window != nil:
		return json.Marshal(t.Window)
	case t.Join != nil:
		return json.Marshal(t.Join)
	case t.Union != nil:
		return json.Marshal(t.Union)
	case t.Pivot != nil:
		return json.Marshal(t.Pivot)
	case t.Unpivot != nil:
		return json.Marshal(t.Unpivot)
	case t.Sample != nil:
		return json.Marshal(t.Sample)
	case t.Sort != nil:
		return json.Marshal(t.Sort)
	case t.Limit != nil:
		return json.Marshal(t.Limit)
	}
	return []byte("null"), nil
}

// UnmarshalJSON inspects the keys present and routes to the matching
// variant. Exactly one discriminator key must be present. Unknown keys
// cause an error via DisallowUnknownFields propagated through
// strictUnmarshal.
func (t *Transform) UnmarshalJSON(data []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("transform: %w", err)
	}
	matched := 0
	hit := ""
	for _, key := range transformDiscriminators {
		if _, ok := probe[key]; ok {
			matched++
			hit = key
		}
	}
	if matched == 0 {
		return fmt.Errorf("transform: missing discriminator key (one of %v required)", transformDiscriminators)
	}
	if matched > 1 {
		return fmt.Errorf("transform: multiple discriminator keys present, exactly one of %v allowed", transformDiscriminators)
	}
	switch hit {
	case "filter":
		var v FilterTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Filter = &v
	case "calculate":
		var v CalculateTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Calculate = &v
	case "aggregate":
		var v AggregateTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Aggregate = &v
	case "bin":
		var v BinTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Bin = &v
	case "window":
		var v WindowTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Window = &v
	case "join":
		var v JoinTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Join = &v
	case "union":
		var v UnionTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Union = &v
	case "pivot":
		var v PivotTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Pivot = &v
	case "unpivot":
		var v UnpivotTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Unpivot = &v
	case "sample":
		var v SampleTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Sample = &v
	case "sort":
		var v SortTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Sort = &v
	case "limit":
		var v LimitTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Limit = &v
	default:
		return fmt.Errorf("transform: unhandled discriminator %q", hit)
	}
	return nil
}

// transformDiscriminators lists the keys that select a transform variant.
var transformDiscriminators = []string{
	"filter", "calculate", "aggregate", "bin", "window",
	"join", "union", "pivot", "unpivot",
	"sample", "sort", "limit",
}

// strictUnmarshal applies DisallowUnknownFields to a single byte slice.
// Centralizing it keeps every nested decoder consistent with spec.Decode.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
