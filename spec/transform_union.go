package spec

import (
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
	case t.Crosstab != nil:
		return json.Marshal(t.Crosstab)
	case t.Regression != nil:
		return json.Marshal(t.Regression)
	case t.TimeUnit != nil:
		return json.Marshal(t.TimeUnit)
	case t.Stack != nil:
		return json.Marshal(t.Stack)
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
	var matches []string
	for _, key := range transformDiscriminators {
		if _, ok := probe[key]; ok {
			matches = append(matches, key)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("transform: missing discriminator key (one of %v required)", transformDiscriminators)
	}
	hit, err := selectDiscriminator(matches)
	if err != nil {
		return err
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
	case "crosstab":
		var v CrosstabTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Crosstab = &v
	case "regression":
		var v RegressionTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Regression = &v
	case "timeunit":
		var v TimeUnitTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.TimeUnit = &v
	case "stack":
		var v StackTransform
		if err := strictUnmarshal(data, &v); err != nil {
			return err
		}
		t.Stack = &v
	default:
		return fmt.Errorf("transform: unhandled discriminator %q", hit)
	}
	return nil
}

// selectDiscriminator picks the owning variant when more than one
// discriminator key is present.
//
// A few variants legitimately carry another variant's discriminator as
// an ordinary optional field — `window` takes a `sort` array, and
// `sort` is itself a discriminator. The probe therefore prefers the
// variant whose *required* key is present: if exactly one match owns
// every other match as a subordinate key, that match wins. Anything
// else is genuinely ambiguous and still errors.
//
// Before E5-S4 this returned an error for a window carrying a sort
// key — a shape schema/v1/transform.schema.json has always
// advertised, so the schema promised a document the decoder refused.
func selectDiscriminator(matches []string) (string, error) {
	if len(matches) == 1 {
		return matches[0], nil
	}
	owner := ""
	for _, candidate := range matches {
		sub := transformSubordinateKeys[candidate]
		ok := true
		for _, other := range matches {
			if other == candidate {
				continue
			}
			if !containsString(sub, other) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if owner != "" {
			// Two variants each claim the other; refuse rather than pick.
			return "", ambiguousDiscriminatorErr(matches)
		}
		owner = candidate
	}
	if owner == "" {
		return "", ambiguousDiscriminatorErr(matches)
	}
	return owner, nil
}

// ambiguousDiscriminatorErr reports an undecidable key combination.
func ambiguousDiscriminatorErr(matches []string) error {
	return fmt.Errorf("transform: multiple discriminator keys present (%v), exactly one of %v allowed", matches, transformDiscriminators)
}

// containsString reports membership without pulling in a generic
// helper for one three-element lookup.
func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// transformSubordinateKeys maps a transform variant's discriminator to
// the other discriminator keys that variant may legally carry as
// ordinary optional fields. Keep it in step with the structs in
// transform.go — a variant that gains such a field needs a row here or
// the decoder will reject the shape the schema advertises.
var transformSubordinateKeys = map[string][]string{
	"window": {"sort"},
}

// transformDiscriminators lists the keys that select a transform variant.
var transformDiscriminators = []string{
	"filter", "calculate", "aggregate", "bin", "window",
	"join", "union", "pivot", "unpivot",
	"sample", "sort", "limit", "crosstab", "regression", "timeunit",
	"stack",
}
