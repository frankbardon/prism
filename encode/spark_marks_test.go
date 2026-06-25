package encode

import "testing"

// TestIsSparkMark pins the chrome-suppressed spark-mark set: the spark
// variants are members, and non-spark marks (notably bullet) are not.
func TestIsSparkMark(t *testing.T) {
	spark := []string{"sparkline", "sparkbar", "winloss", "sparkarea"}
	for _, m := range spark {
		if !isSparkMark(m) {
			t.Errorf("isSparkMark(%q) = false, want true", m)
		}
	}

	notSpark := []string{"bullet", "bar", "line", "area", "point", "rule", ""}
	for _, m := range notSpark {
		if isSparkMark(m) {
			t.Errorf("isSparkMark(%q) = true, want false", m)
		}
	}
}
