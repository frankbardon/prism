package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// OrderChannelAggregate implements PRISM_SPEC_056: an `encoding.order`
// entry may not declare its own `aggregate`.
//
// The decision, recorded here because the story asked for it either
// way: Prism rejects rather than honours. Vega-Lite reads
// `{"order": {"aggregate": "sum", "field": "revenue"}}` as "order the
// series by each series' total", which needs a second aggregation at a
// different granularity than the chart's own, joined back onto the
// rows. That is a whole machine, and half-building it is the silent
// no-op this effort exists to eliminate — the key would be accepted
// and quietly ignored.
//
// The supported ways to express the same intent:
//
//   - bind order to a column that already carries the total, computed
//     upstream with a `window` or `join` transform;
//   - bind order to the field another channel already aggregates, in
//     which case the order key reads that aggregate's output column
//     (plan/build's injectEncodingAggregate on purpose does not widen
//     the groupby for it);
//   - shape row order with a `sort` transform and leave `order` unbound.
type OrderChannelAggregate struct{}

// Code returns PRISM_SPEC_056.
func (OrderChannelAggregate) Code() string { return "PRISM_SPEC_056" }

// Check walks the order channel on the root spec and on every
// composition child that can carry its own encoding.
func (OrderChannelAggregate) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walkScaleDomainSpecs(s, func(sub *spec.Spec) {
		out = append(out, checkOrderChannelAggregate(sub.Encoding)...)
	})
	return out
}

func checkOrderChannelAggregate(enc *spec.Encoding) []*errors.AppError {
	var out []*errors.AppError
	for i, e := range spec.OrderEntries(enc) {
		if e.Aggregate == "" {
			continue
		}
		key := fmt.Sprintf("order[%d]", i)
		out = append(out, errors.New("PRISM_SPEC_056",
			fmt.Sprintf("Order entry %s declares aggregate %q, which Prism does not support on the order channel.", key, e.Aggregate),
			map[string]any{"Entry": key, "Field": e.Field, "Aggregate": e.Aggregate},
		))
	}
	return out
}
