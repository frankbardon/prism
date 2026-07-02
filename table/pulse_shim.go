package table

import "github.com/frankbardon/pulse/encoding"

// TEMP: deleted in E4.
//
// This file is the ONLY Pulse dependency in the native schema surface. It
// bridges github.com/frankbardon/pulse/encoding.{Schema,Field,FieldType,
// Dictionary} to the Prism-native types so leaf executors that still open
// Pulse in epic E1 (source / crosstab / pulse_chain / regression) can hand
// their Pulse-produced schema to a native table.Table during the E1-S2/E1-S3
// migration. Once the Pulse loader is removed in epic E4, delete this file
// and the whole table/ package becomes Pulse-free.
//
// The FieldType byte layout is identical to Pulse's, so the type
// conversion is a direct cast; the helpers exist to convert the surrounding
// struct shapes (Dictionary is opaque in Pulse, so it is rebuilt via its
// public Values/Add surface).

// FromPulseFieldType converts a Pulse FieldType to the native FieldType.
//
// TEMP: deleted in E4.
func FromPulseFieldType(ft encoding.FieldType) FieldType { return FieldType(ft) }

// ToPulseFieldType converts a native FieldType to the Pulse FieldType.
//
// TEMP: deleted in E4.
func ToPulseFieldType(ft FieldType) encoding.FieldType { return encoding.FieldType(ft) }

// FromPulseSchema converts a *encoding.Schema to a native *Schema. Returns
// nil for a nil input. Categorical dictionaries are rebuilt from the Pulse
// dictionary's ordered Values so IDs line up.
//
// TEMP: deleted in E4.
func FromPulseSchema(s *encoding.Schema) *Schema {
	if s == nil {
		return nil
	}
	out := &Schema{Fields: make([]Field, len(s.Fields))}
	for i := range s.Fields {
		out.Fields[i] = fromPulseField(&s.Fields[i])
	}
	return out
}

// ToPulseSchema converts a native *Schema back to a *encoding.Schema.
// Returns nil for a nil input.
//
// TEMP: deleted in E4.
func ToPulseSchema(s *Schema) *encoding.Schema {
	if s == nil {
		return nil
	}
	out := &encoding.Schema{Fields: make([]encoding.Field, len(s.Fields))}
	for i := range s.Fields {
		out.Fields[i] = toPulseField(&s.Fields[i])
	}
	return out
}

func fromPulseField(f *encoding.Field) Field {
	return Field{
		Name:        f.Name,
		Type:        FromPulseFieldType(f.Type),
		Nullable:    f.Nullable,
		Dictionary:  fromPulseDictionary(f.Dictionary),
		Precision:   f.Precision,
		Scale:       f.Scale,
		Description: f.Description,
	}
}

func toPulseField(f *Field) encoding.Field {
	return encoding.Field{
		Name:        f.Name,
		Type:        ToPulseFieldType(f.Type),
		Nullable:    f.Nullable,
		Dictionary:  toPulseDictionary(f.Dictionary),
		Precision:   f.Precision,
		Scale:       f.Scale,
		Description: f.Description,
	}
}

func fromPulseDictionary(d *encoding.Dictionary) *Dictionary {
	if d == nil {
		return nil
	}
	nd := NewDictionary()
	for _, v := range d.Values() {
		_, _ = nd.Add(v)
	}
	return nd
}

func toPulseDictionary(d *Dictionary) *encoding.Dictionary {
	if d == nil {
		return nil
	}
	pd := encoding.NewDictionary()
	for _, v := range d.Values() {
		_, _ = pd.Add(v)
	}
	return pd
}
