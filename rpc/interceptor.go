//go:build !js

// Error interceptor: maps *prismerrors.AppError (and unknown errors)
// to Twirp status codes by code prefix. See D085 for the full mapping
// table.
//
// Original PRISM_* code is preserved as meta on the returned
// twirp.Error under the "code" key. AppError fixups + context attach
// as "fixups" and "context".

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/twitchtv/twirp"

	prismerrors "github.com/frankbardon/prism/errors"
)

// ErrorInterceptor is a twirp.Interceptor that turns every non-Twirp
// error returned from a handler into a typed twirp.Error.
// twirp.Error values pass through unchanged.
func ErrorInterceptor(next twirp.Method) twirp.Method {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		resp, err := next(ctx, req)
		if err == nil {
			return resp, nil
		}
		// Already a Twirp error → pass through.
		var twerr twirp.Error
		if errors.As(err, &twerr) {
			return resp, err
		}
		return resp, toTwirpError(err)
	}
}

// toTwirpError translates one application error to a twirp.Error.
// Public so callers (and tests) can exercise the mapping outside
// the interceptor chain.
func toTwirpError(err error) twirp.Error {
	if err == nil {
		return nil
	}

	// *prismerrors.AppError → mapping by PRISM_ prefix.
	var ae *prismerrors.AppError
	if errors.As(err, &ae) {
		code := twirpStatusForCode(ae.Code)
		out := twirp.NewError(code, ae.Message).
			WithMeta("code", ae.Code)
		if len(ae.Fixups) > 0 {
			out = out.WithMeta("fixups", strings.Join(ae.Fixups, "\n"))
		}
		if len(ae.Context) > 0 {
			if b, mErr := json.Marshal(ae.Context); mErr == nil {
				out = out.WithMeta("context", string(b))
			}
		}
		return out
	}

	// Unknown — wrap as Internal.
	return twirp.InternalError(err.Error())
}

// twirpStatusForCode is the prefix → status table (D085). Returns
// twirp.Internal for unknown codes.
func twirpStatusForCode(code string) twirp.ErrorCode {
	switch {
	case code == "PRISM_RENDER_FORMAT_UNAVAILABLE":
		return twirp.Unimplemented

	// PRISM domain.
	case strings.HasPrefix(code, "PRISM_SPEC_"):
		return twirp.InvalidArgument
	case strings.HasPrefix(code, "PRISM_SCHEMA_"):
		return twirp.InvalidArgument
	case strings.HasPrefix(code, "PRISM_VALIDATE_"):
		return twirp.InvalidArgument

	case code == "PRISM_PLAN_002":
		// PLAN_002 = transform refs undefined dataset → NotFound.
		return twirp.NotFound
	case strings.HasPrefix(code, "PRISM_RESOLVE_"):
		return twirp.NotFound

	case strings.HasPrefix(code, "PRISM_COMPILE_"),
		strings.HasPrefix(code, "PRISM_PLAN_"),
		strings.HasPrefix(code, "PRISM_JOIN_"),
		strings.HasPrefix(code, "PRISM_EXEC_"),
		strings.HasPrefix(code, "PRISM_ENCODE_"):
		return twirp.Internal

	// SERVE codes: decode + selection synthesis are user errors;
	// execute/encode are server-side. Decide by suffix.
	case code == "PRISM_SERVE_DECODE",
		code == "PRISM_SERVE_SELECTION",
		code == "PRISM_SERVE_METHOD",
		code == "PRISM_SERVE_READ":
		return twirp.InvalidArgument
	case strings.HasPrefix(code, "PRISM_SERVE_"):
		return twirp.Internal

	case strings.HasPrefix(code, "PRISM_RENDER_"),
		strings.HasPrefix(code, "PRISM_WARN_"):
		return twirp.Internal
	}

	// Unknown / non-PRISM code → Internal.
	return twirp.Internal
}
