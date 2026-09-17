package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// OrderChannelShape implements PRISM_SPEC_055: every `encoding.order`
// entry must name a field, and its `sort` direction must be one Prism
// recognises.
//
// Both halves close a silent no-op. An entry with no field has nothing
// to compare, so the plan's synthetic SortNode would skip it and the
// author's stated ordering would simply not happen. An unrecognised
// direction is worse: spec.SortDirectionDescending reports false for
// anything it does not know, so a typo'd "DESC" or "reverse" would
// sort ascending and look intentional.
//
// The canonical directions are "ascending" and "descending"; "asc" and
// "desc" are accepted aliases (see spec/order.go). Both spellings are
// in the JSON Schema enum, so the schema and this rule agree.
type OrderChannelShape struct{}

// Code returns PRISM_SPEC_055.
func (OrderChannelShape) Code() string { return "PRISM_SPEC_055" }

// Check walks the order channel on the root spec and on every
// composition child that can carry its own encoding.
func (OrderChannelShape) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walkScaleDomainSpecs(s, func(sub *spec.Spec) {
		out = append(out, checkOrderChannelShape(sub.Encoding)...)
	})
	return out
}

func checkOrderChannelShape(enc *spec.Encoding) []*errors.AppError {
	var out []*errors.AppError
	for i, e := range spec.OrderEntries(enc) {
		key := fmt.Sprintf("order[%d]", i)
		if e.Field == "" {
			out = append(out, errors.New("PRISM_SPEC_055",
				fmt.Sprintf("Order entry %s declares no field; there is nothing to order rows by.", key),
				map[string]any{"Entry": key},
			))
			continue
		}
		if !spec.SortDirectionValid(e.Sort) {
			out = append(out, errors.New("PRISM_SPEC_055",
				fmt.Sprintf("Order entry %s declares sort %q; expected \"ascending\" or \"descending\".", key, e.Sort),
				map[string]any{"Entry": key, "Field": e.Field, "Sort": e.Sort},
			))
		}
	}
	return out
}
