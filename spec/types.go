// Package spec contains the Go types that mirror the Prism v1 JSON schema
// bundle (schema/v1/*.json). Decoding is strict: unknown fields fail. Use
// Decode to read a spec from an io.Reader; it sets DisallowUnknownFields and
// returns the typed *Spec.
//
// Type structure intentionally uses pointer-to-struct for optional nested
// blocks so omitempty works correctly with json.Marshal.
package spec

// Spec is the top-level Prism chart specification. Maps 1:1 to
// schema/v1/spec.schema.json. Exactly one of mark/layer/concat/hconcat/
// vconcat/facet/repeat must be set; the JSON Schema layer enforces this.
type Spec struct {
	Schema      string               `json:"$schema"`
	Data        *Data                `json:"data,omitempty"`
	Datasets    map[string]*Data     `json:"datasets,omitempty"`
	Transform   []Transform          `json:"transform,omitempty"`
	Mark        *Mark                `json:"mark,omitempty"`
	Encoding    *Encoding            `json:"encoding,omitempty"`
	Layer       []*Spec              `json:"layer,omitempty"`
	Concat      []*Spec              `json:"concat,omitempty"`
	HConcat     []*Spec              `json:"hconcat,omitempty"`
	VConcat     []*Spec              `json:"vconcat,omitempty"`
	Facet       *Facet               `json:"facet,omitempty"`
	Repeat      *Repeat              `json:"repeat,omitempty"`
	ChildSpec   *Spec                `json:"spec,omitempty"`
	Selection   map[string]Selection `json:"selection,omitempty"`
	Resolve     *Resolve             `json:"resolve,omitempty"`
	Theme       *ThemeOverride       `json:"theme,omitempty"`
	Width       *Dimension           `json:"width,omitempty"`
	Height      *Dimension           `json:"height,omitempty"`
	Padding     *Padding             `json:"padding,omitempty"`
	Background  string               `json:"background,omitempty"`
	Title       *TextOrTextObj       `json:"title,omitempty"`
	Subtitle    *TextOrTextObj       `json:"subtitle,omitempty"`
	Description string               `json:"description,omitempty"`
	Projection  *Projection          `json:"projection,omitempty"`
	Animation   *Animation           `json:"animation,omitempty"`
}
