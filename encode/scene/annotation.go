package scene

import "encoding/json"

// AnnotationType is the discriminator for annotation payloads.
// Underspecified intentionally in v1 — text + line are common; richer
// types (region, arrow) land when fixtures demand.
type AnnotationType string

const (
	AnnotationText   AnnotationType = "text"
	AnnotationLine   AnnotationType = "line"
	AnnotationRegion AnnotationType = "region"
	AnnotationArrow  AnnotationType = "arrow"
)

// Annotation carries a typed payload as raw JSON. The renderer
// dispatches on Type and decodes Data accordingly.
type Annotation struct {
	Type AnnotationType  `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}
