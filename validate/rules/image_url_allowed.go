package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ImageURLAllowed implements PRISM_SPEC_016: image marks may only
// reference data: URLs or relative paths. Remote URLs (http, https,
// ftp, file, gs, s3, ...) are rejected at validate time per D068 so
// the encoder + renderer stay network-free (matches PROJECT.md
// offline-first principle).
type ImageURLAllowed struct{}

// Code returns PRISM_SPEC_016.
func (ImageURLAllowed) Code() string { return "PRISM_SPEC_016" }

// Check fires when an image mark's URL string is non-empty AND
// is neither a data: URL nor a relative path.
func (ImageURLAllowed) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil {
		return nil
	}
	if s.Mark.TypeName() != "image" {
		return nil
	}
	url := ""
	if s.Mark.Def != nil {
		url = s.Mark.Def.URL
	}
	if url == "" {
		// Empty URL handled by a different rule (mark requires url
		// for image — out of scope here; image encoder also defends).
		return nil
	}
	if isAllowedImageURL(url) {
		return nil
	}
	return []*errors.AppError{
		errors.New("PRISM_SPEC_016",
			fmt.Sprintf("Image URL %q is not allowed (offline-first; only data: and relative paths are accepted).", url),
			map[string]any{"URL": url},
		),
	}
}

// isAllowedImageURL returns true for data: URLs and relative paths
// (no scheme, no leading "/"). Any string containing "://" or
// starting with one of the listed schemes is rejected.
func isAllowedImageURL(url string) bool {
	if strings.HasPrefix(url, "data:") {
		return true
	}
	// Disallow any URL with a scheme separator.
	if strings.Contains(url, "://") {
		return false
	}
	// Disallow common scheme prefixes (these include a colon but
	// may not be followed by "//" in the rare absolute-form case;
	// rejection here is defensive belt-and-suspenders).
	for _, prefix := range disallowedSchemePrefixes {
		if strings.HasPrefix(strings.ToLower(url), prefix) {
			return false
		}
	}
	// Absolute filesystem paths (leading "/") are rejected — the
	// renderer's working directory anchors relative paths, and
	// absolute paths leak outside the spec's workspace.
	if strings.HasPrefix(url, "/") {
		return false
	}
	return true
}

var disallowedSchemePrefixes = []string{
	"http:", "https:", "ftp:", "ftps:",
	"file:", "ws:", "wss:", "mailto:",
	"gs:", "s3:", "azure:", "gcs:",
}
