package scene

// SceneLayer is one Pulse-query-worth of marks. Layers stack
// back-to-front per ZIndex; renderer emits one <g> per layer for DOM
// addressability + selection styling.
type SceneLayer struct {
	ID      string   `json:"id"`
	Source  string   `json:"source,omitempty"`
	Mark    MarkType `json:"mark"`
	Marks   []Mark   `json:"marks"`
	Clip    bool     `json:"clip,omitempty"`
	Opacity float64  `json:"opacity,omitempty"`
	ZIndex  int      `json:"z_index,omitempty"`
	Hidden  bool     `json:"hidden,omitempty"`
}
