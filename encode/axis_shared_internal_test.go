package encode

import (
	"reflect"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// axisProbeValues supplies two distinct non-zero values for each Go
// kind the spec.Axis struct uses, so the coverage test below can drive
// *every* property through the merge without hand-listing them.
func axisProbeValues(t *testing.T, f reflect.StructField) (reflect.Value, reflect.Value) {
	t.Helper()
	switch f.Type.Kind() {
	case reflect.String:
		return reflect.ValueOf("alpha").Convert(f.Type), reflect.ValueOf("omega").Convert(f.Type)
	case reflect.Interface:
		a := reflect.New(f.Type).Elem()
		a.Set(reflect.ValueOf("alpha"))
		b := reflect.New(f.Type).Elem()
		b.Set(reflect.ValueOf("omega"))
		return a, b
	case reflect.Slice:
		a := reflect.MakeSlice(f.Type, 1, 1)
		b := reflect.MakeSlice(f.Type, 1, 1)
		ea, eb := axisProbeElem(t, f.Type.Elem())
		a.Index(0).Set(ea)
		b.Index(0).Set(eb)
		return a, b
	case reflect.Pointer:
		a := reflect.New(f.Type.Elem())
		b := reflect.New(f.Type.Elem())
		ea, eb := axisProbeElem(t, f.Type.Elem())
		a.Elem().Set(ea)
		b.Elem().Set(eb)
		return a, b
	}
	t.Fatalf("spec.Axis.%s has unsupported kind %s; extend axisProbeValues", f.Name, f.Type.Kind())
	return reflect.Value{}, reflect.Value{}
}

func axisProbeElem(t *testing.T, ty reflect.Type) (reflect.Value, reflect.Value) {
	t.Helper()
	switch ty.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(true).Convert(ty), reflect.ValueOf(false).Convert(ty)
	case reflect.Int, reflect.Int64:
		return reflect.ValueOf(int64(3)).Convert(ty), reflect.ValueOf(int64(7)).Convert(ty)
	case reflect.Float64:
		return reflect.ValueOf(3.0).Convert(ty), reflect.ValueOf(7.0).Convert(ty)
	case reflect.String:
		return reflect.ValueOf("alpha").Convert(ty), reflect.ValueOf("omega").Convert(ty)
	case reflect.Interface:
		a := reflect.New(ty).Elem()
		a.Set(reflect.ValueOf("alpha"))
		b := reflect.New(ty).Elem()
		b.Set(reflect.ValueOf("omega"))
		return a, b
	}
	t.Fatalf("unsupported element kind %s; extend axisProbeElem", ty.Kind())
	return reflect.Value{}, reflect.Value{}
}

// TestPrismSharedAxisMergeCoversEveryProperty drives every exported
// property of spec.Axis through mergeSharedAxisSpec. It pins two
// things at once: a property specified by one child alone survives the
// merge, and two children disagreeing on it raise a conflict warning
// rather than being silently resolved. Adding a property to spec.Axis
// therefore needs no second registration site — but if the merge ever
// stops being exhaustive, this fails.
func TestPrismSharedAxisMergeCoversEveryProperty(t *testing.T) {
	typ := reflect.TypeOf(spec.Axis{})
	if typ.NumField() == 0 {
		t.Fatal("spec.Axis has no fields")
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue // unexported
		}
		t.Run(f.Name, func(t *testing.T) {
			first, second := axisProbeValues(t, f)

			a := &spec.Axis{}
			reflect.ValueOf(a).Elem().Field(i).Set(first)
			b := &spec.Axis{}
			reflect.ValueOf(b).Elem().Field(i).Set(second)

			// Only the first child specifies: it wins, no conflict.
			merged, warns := mergeSharedAxisSpec(scene.ChannelX, []sharedAxisBlock{
				{Label: "layer-0", Axis: a},
				{Label: "layer-1", Axis: &spec.Axis{}},
			})
			if merged == nil {
				t.Fatalf("%s: merge dropped the only specified property", f.Name)
			}
			got := reflect.ValueOf(merged).Elem().Field(i).Interface()
			if !reflect.DeepEqual(got, first.Interface()) {
				t.Errorf("%s: merged = %v, want %v", f.Name, got, first.Interface())
			}
			if len(warns) != 0 {
				t.Errorf("%s: unexpected warnings %+v", f.Name, warns)
			}

			// Both children specify, and disagree: first wins, conflict warns.
			merged, warns = mergeSharedAxisSpec(scene.ChannelX, []sharedAxisBlock{
				{Label: "layer-0", Axis: a},
				{Label: "layer-1", Axis: b},
			})
			got = reflect.ValueOf(merged).Elem().Field(i).Interface()
			if !reflect.DeepEqual(got, first.Interface()) {
				t.Errorf("%s: conflict merged = %v, want first-specified %v", f.Name, got, first.Interface())
			}
			if len(warns) != 1 {
				t.Fatalf("%s: warnings = %d, want 1 conflict; got %+v", f.Name, len(warns), warns)
			}
			if warns[0].Code != scene.WarnAxisConfigConflict {
				t.Errorf("%s: warning code = %q, want %q", f.Name, warns[0].Code, scene.WarnAxisConfigConflict)
			}
			if want := axisPropertyName(f); warns[0].Details["Property"] != want {
				t.Errorf("%s: warning property = %v, want %q", f.Name, warns[0].Details["Property"], want)
			}

			// Agreement on the same value is not a conflict.
			if _, warns = mergeSharedAxisSpec(scene.ChannelX, []sharedAxisBlock{
				{Label: "layer-0", Axis: a},
				{Label: "layer-1", Axis: a},
			}); len(warns) != 0 {
				t.Errorf("%s: agreeing children warned: %+v", f.Name, warns)
			}
		})
	}
}

func TestPrismSharedAxisMergeEmptyYieldsNoBlock(t *testing.T) {
	merged, warns := mergeSharedAxisSpec(scene.ChannelY, nil)
	if merged != nil {
		t.Errorf("merged = %+v, want nil when no child specifies axis config", merged)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %+v, want none", warns)
	}
	merged, _ = mergeSharedAxisSpec(scene.ChannelY, []sharedAxisBlock{{Label: "layer-0", Axis: &spec.Axis{}}})
	if merged != nil {
		t.Errorf("merged = %+v, want nil for an empty axis block", merged)
	}
}

func TestPrismSharedAxisOptsFallsBackToDefaults(t *testing.T) {
	opts, warns := sharedAxisOpts(scene.ChannelX, nil, "day")
	if len(warns) != 0 {
		t.Errorf("warnings = %+v, want none", warns)
	}
	if want := DefaultAxisOpts("day"); opts != want {
		t.Errorf("opts = %+v, want %+v", opts, want)
	}
}
