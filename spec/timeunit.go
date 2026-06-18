package spec

// TimeUnitTransform truncates a temporal field to a calendar period and
// appends the truncated date as a new column — the Vega-Lite `timeUnit`
// analogue. Output is a date (period start), so the derived column
// stays temporal for axis / scale resolution and sorts chronologically.
//
// Runs client-side (pure epoch arithmetic, like `bin`) — no Pulse leaf,
// no first-transform constraint, composes anywhere in a chain.
//
// `timeunit` is the unit string and the discriminator: one of year,
// quarter, month, week (ISO, Monday start), day.
type TimeUnitTransform struct {
	TimeUnit string `json:"timeunit"`
	Field    string `json:"field"`
	As       string `json:"as"`
	Data     string `json:"data,omitempty"`
}

// TimeUnits lists the supported calendar periods for a timeunit
// transform. All truncate to the period start and emit a date.
var TimeUnits = []string{"year", "quarter", "month", "week", "day"}
