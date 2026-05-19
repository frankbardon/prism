package scene

// SelectionKind is the discriminator between point and interval
// selections. P05 ships the types; selection wiring lands in P13.
type SelectionKind string

const (
	SelectionPoint    SelectionKind = "point"
	SelectionInterval SelectionKind = "interval"
)

// SelectionEvent describes the user gesture that triggers a selection.
type SelectionEvent string

const (
	EventClick    SelectionEvent = "click"
	EventHover    SelectionEvent = "hover"
	EventBrush    SelectionEvent = "brush"
	EventLasso    SelectionEvent = "lasso"
	EventDblclick SelectionEvent = "dblclick"
)

// ResolveStrategy controls how a selection is resolved across layers.
type ResolveStrategy string

const (
	ResolveGlobal      ResolveStrategy = "global"
	ResolveIndependent ResolveStrategy = "independent"
	ResolveUnion       ResolveStrategy = "union"
	ResolveIntersect   ResolveStrategy = "intersect"
)

// Reactive controls where a selection's filtering loop runs.
type Reactive string

const (
	ReactiveClient Reactive = "client"
	ReactiveServer Reactive = "server"
	ReactiveBoth   Reactive = "both"
)

// Selection is the post-resolve selection descriptor.
type Selection struct {
	ID        string          `json:"id"`
	Kind      SelectionKind   `json:"kind"`
	Channels  []Channel       `json:"channels,omitempty"`
	On        SelectionEvent  `json:"on,omitempty"`
	Resolve   ResolveStrategy `json:"resolve,omitempty"`
	Encodings []string        `json:"encodings,omitempty"`
	InitState *SelectionState `json:"init_state,omitempty"`
	Reactive  Reactive        `json:"reactive,omitempty"`
}

// SelectionState carries the active selection's data (post-event).
type SelectionState struct {
	Points []DatumRef      `json:"points,omitempty"`
	Range  *SelectionRange `json:"range,omitempty"`
}

// SelectionRange is the value-space bounds of an interval selection.
type SelectionRange struct {
	X [2]float64 `json:"x,omitempty"`
	Y [2]float64 `json:"y,omitempty"`
}
