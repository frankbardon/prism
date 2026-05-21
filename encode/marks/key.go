package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
)

// AttachKeys stamps Mark.Key on per-row marks for use by the
// client-side animator. The key string format is "<field>=<value>";
// missing or out-of-range rows yield an empty key (mark falls back
// to positional matching at tween time).
//
// Like AttachDatum, only the leading rowCount marks are stamped;
// composite encoders that emit trailing helper marks (boxplot
// whiskers, histogram bin edges) are left untouched.
func AttachKeys(marks []scene.Mark, tbl *table.Table, field string) {
	if len(marks) == 0 || tbl == nil || field == "" {
		return
	}
	col, ok := tbl.Column(field)
	if !ok {
		return
	}
	n := min(col.Len(), len(marks))
	for i := range n {
		marks[i].Key = fmt.Sprintf("%s=%v", field, col.ValueAt(i))
	}
}
