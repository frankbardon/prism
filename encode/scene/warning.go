package scene

// Warning is the structured-warning shape attached to SceneDoc and
// surfaced by the CLI / browser. Codes use the PRISM_WARN_* form.
type Warning struct {
	Code    string         `json:"code"`
	Layer   string         `json:"layer,omitempty"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Known warning codes emitted by the encoder / renderer in P05.
const (
	WarnTimeScaleStubbed     = "PRISM_WARN_TIME_SCALE_STUBBED"
	WarnMarkNotImplemented   = "PRISM_WARN_MARK_NOT_IMPLEMENTED"
	WarnNoDataForLayer       = "PRISM_WARN_NO_DATA_FOR_LAYER"
	WarnPrecisionTruncation  = "PRISM_WARN_PRECISION_TRUNCATION"
)
