package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConditionUnmarshalSingle(t *testing.T) {
	src := `{"selection":"brush","value":"#22c55e"}`
	var c Condition
	if err := json.Unmarshal([]byte(src), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Single == nil || c.Multi != nil {
		t.Fatalf("expected single, got %+v", c)
	}
	if c.Single.Selection != "brush" || c.Single.Value != "#22c55e" {
		t.Errorf("fields mismatch: %+v", c.Single)
	}
}

func TestConditionUnmarshalMulti(t *testing.T) {
	src := `[
		{"selection":"brush","field":"region","type":"nominal"},
		{"test":{"op":"gte","field":"score","value":0.7},"value":"#22c55e"}
	]`
	var c Condition
	if err := json.Unmarshal([]byte(src), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Multi == nil || len(c.Multi) != 2 {
		t.Fatalf("expected multi[2], got %+v", c)
	}
	if c.Multi[0].Selection != "brush" || c.Multi[0].Field != "region" {
		t.Errorf("first entry: %+v", c.Multi[0])
	}
	if c.Multi[1].Test == nil || c.Multi[1].Test.Op != PredGte || c.Multi[1].Test.Field != "score" {
		t.Errorf("second entry test: %+v", c.Multi[1].Test)
	}
	if c.Multi[1].Value != "#22c55e" {
		t.Errorf("second entry value: %+v", c.Multi[1].Value)
	}
}

func TestConditionRoundTripSingle(t *testing.T) {
	c := Condition{Single: &ConditionTest{Test: &Predicate{Op: PredGt, Field: "x", Value: 0.0}, Value: "red"}}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Condition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Single == nil || got.Single.Test == nil || got.Single.Test.Op != PredGt || got.Single.Value != "red" {
		t.Errorf("round-trip mismatch: %+v", got.Single)
	}
}

func TestConditionRoundTripMulti(t *testing.T) {
	c := Condition{Multi: []ConditionTest{
		{Selection: "brush", Field: "region", Type: "nominal"},
		{Test: &Predicate{Op: PredGte, Field: "score", Value: 0.7}, Value: "#22c55e"},
	}}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Condition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Multi) != 2 || got.Multi[1].Test == nil || got.Multi[1].Test.Field != "score" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestConditionInvalid(t *testing.T) {
	var c Condition
	err := json.Unmarshal([]byte(`"plain-string"`), &c)
	if err == nil || !strings.Contains(err.Error(), "expected object or array") {
		t.Errorf("expected reject, got err=%v", err)
	}
}

// TestConditionTestRejectsExpressionString — the legacy free-form
// expression string form of `test` is rejected at decode time by
// Predicate.UnmarshalJSON (E2-S3).
func TestConditionTestRejectsExpressionString(t *testing.T) {
	var c Condition
	err := json.Unmarshal([]byte(`{"test":"score >= 0.7","value":"green"}`), &c)
	if err == nil || !strings.Contains(err.Error(), "structured object") {
		t.Errorf("expected structured-predicate rejection, got err=%v", err)
	}
}

func TestConditionEntries(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		c := &Condition{Single: &ConditionTest{Selection: "brush"}}
		entries := c.Entries()
		if len(entries) != 1 || entries[0].Selection != "brush" {
			t.Errorf("got %+v", entries)
		}
	})
	t.Run("multi", func(t *testing.T) {
		c := &Condition{Multi: []ConditionTest{
			{Test: &Predicate{Op: PredGt, Field: "a", Value: 0.0}},
			{Test: &Predicate{Op: PredLt, Field: "b", Value: 0.0}},
		}}
		entries := c.Entries()
		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}
	})
	t.Run("nil", func(t *testing.T) {
		var c *Condition
		if got := c.Entries(); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestChannelCommonWithCondition(t *testing.T) {
	src := `{
		"field":"score",
		"type":"quantitative",
		"condition":[
			{"selection":"brush","value":"#22c55e"},
			{"test":{"op":"lt","field":"score","value":0},"value":"#ef4444"}
		],
		"value":"#cbd5e1"
	}`
	var ch MarkChannel
	if err := json.Unmarshal([]byte(src), &ch); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ch.Condition == nil || len(ch.Condition.Multi) != 2 {
		t.Fatalf("condition not parsed: %+v", ch.Condition)
	}
	if ch.Condition.Multi[1].Test == nil || ch.Condition.Multi[1].Test.Op != PredLt {
		t.Errorf("test predicate not parsed: %+v", ch.Condition.Multi[1].Test)
	}
	if ch.Value != "#cbd5e1" {
		t.Errorf("fallback value: %v", ch.Value)
	}
}
