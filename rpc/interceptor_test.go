package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	pulseerrors "github.com/frankbardon/pulse/errors"
	"github.com/twitchtv/twirp"

	prismerrors "github.com/frankbardon/prism/errors"
)

// TestPrismErrorInterceptorStatusCodes is one of the four
// PHASE.md-mandated P14 test gates. Table-driven across one error
// per PRISM_* family + the Pulse domains. For each entry, run the
// error through ErrorInterceptor and assert the returned
// twirp.Error.Code() matches.
func TestPrismErrorInterceptorStatusCodes(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		want   twirp.ErrorCode
		hasMsg string // substring of expected message; empty = skip
	}{
		// PRISM domain
		{"spec", prismerrors.New("PRISM_SPEC_001", "spec err", nil), twirp.InvalidArgument, "spec err"},
		{"validate", prismerrors.New("PRISM_VALIDATE_001", "validate err", nil), twirp.InvalidArgument, ""},
		{"schema", prismerrors.New("PRISM_SCHEMA_INIT", "schema err", nil), twirp.InvalidArgument, ""},
		{"resolve", prismerrors.New("PRISM_RESOLVE_001", "resolve err", nil), twirp.NotFound, ""},
		{"plan002", prismerrors.New("PRISM_PLAN_002", "ref undef", nil), twirp.NotFound, ""},
		{"plan001", prismerrors.New("PRISM_PLAN_001", "cycle", nil), twirp.Internal, ""},
		{"compile", prismerrors.New("PRISM_COMPILE_001", "compile err", nil), twirp.Internal, ""},
		{"join", prismerrors.New("PRISM_JOIN_001", "join err", nil), twirp.Internal, ""},
		{"exec", prismerrors.New("PRISM_EXEC_001", "exec err", nil), twirp.Internal, ""},
		{"encode", prismerrors.New("PRISM_ENCODE_001", "encode err", nil), twirp.Internal, ""},
		{"render_fmt", prismerrors.New("PRISM_RENDER_FORMAT_UNAVAILABLE", "png unavail", nil), twirp.Unimplemented, "unavail"},
		{"render_other", prismerrors.New("PRISM_RENDER_FAILED", "render err", nil), twirp.Internal, ""},
		{"serve_decode", prismerrors.New("PRISM_SERVE_DECODE", "decode err", nil), twirp.InvalidArgument, ""},
		{"serve_selection", prismerrors.New("PRISM_SERVE_SELECTION", "sel err", nil), twirp.InvalidArgument, ""},
		{"serve_method", prismerrors.New("PRISM_SERVE_METHOD", "method err", nil), twirp.InvalidArgument, ""},
		{"serve_execute", prismerrors.New("PRISM_SERVE_EXECUTE", "exec err", nil), twirp.Internal, ""},
		{"serve_encode", prismerrors.New("PRISM_SERVE_ENCODE", "encode err", nil), twirp.Internal, ""},
		{"warn", prismerrors.New("PRISM_WARN_DOWNSAMPLE", "warn", nil), twirp.Internal, ""},
		// PULSE domain (domain-word prefixes per the Pulse catalog)
		{"pulse_data_file", pulseerrors.NewCodedError(pulseerrors.DATA_FILE, "file not found"), twirp.Unavailable, "file"},
		{"pulse_data_parse", pulseerrors.NewCodedError(pulseerrors.DATA_PARSE, "parse err"), twirp.InvalidArgument, ""},
		{"pulse_processing", pulseerrors.NewCodedError(pulseerrors.PROCESSING_RUNTIME, "runtime err"), twirp.Internal, ""},
		{"pulse_encoding_invalid", pulseerrors.NewCodedError(pulseerrors.ENCODING_INVALID, "bad bytes"), twirp.InvalidArgument, ""},
		{"pulse_service_validation", pulseerrors.NewCodedError(pulseerrors.SERVICE_VALIDATION, "bad cfg"), twirp.InvalidArgument, ""},
		// Unknown error
		{"unknown", errors.New("something went wrong"), twirp.Internal, "wrong"},
		// Unknown PRISM code → defaults to Internal
		{"unknown_prism", prismerrors.New("PRISM_FUTURE_001", "from the future", nil), twirp.Internal, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, tc.err
			}
			intercepted := ErrorInterceptor(handler)
			_, got := intercepted(context.Background(), nil)
			if got == nil {
				t.Fatalf("interceptor returned nil error; want code %v", tc.want)
			}
			var twerr twirp.Error
			if !errors.As(got, &twerr) {
				t.Fatalf("interceptor returned non-twirp error: %v", got)
			}
			if twerr.Code() != tc.want {
				t.Fatalf("twirp code = %v; want %v (err=%v)", twerr.Code(), tc.want, got)
			}
			if tc.hasMsg != "" && !strings.Contains(twerr.Msg(), tc.hasMsg) {
				t.Fatalf("twirp msg %q missing substring %q", twerr.Msg(), tc.hasMsg)
			}
		})
	}
}

// TestPrismErrorInterceptorPreservesMetadata ensures the interceptor
// attaches the original PRISM_/PULSE_ code as a meta entry so
// clients keep structured access.
func TestPrismErrorInterceptorPreservesMetadata(t *testing.T) {
	ae := prismerrors.New("PRISM_SPEC_001", "field missing", map[string]any{"Field": "x"})
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, ae
	}
	_, got := ErrorInterceptor(handler)(context.Background(), nil)
	var twerr twirp.Error
	if !errors.As(got, &twerr) {
		t.Fatalf("not a twirp error: %v", got)
	}
	if twerr.Meta("code") != "PRISM_SPEC_001" {
		t.Fatalf("meta.code = %q; want PRISM_SPEC_001", twerr.Meta("code"))
	}
	if twerr.Meta("context") == "" {
		t.Fatalf("meta.context empty")
	}
	if !strings.Contains(twerr.Meta("context"), `"Field":"x"`) {
		t.Fatalf("meta.context = %q; missing Field", twerr.Meta("context"))
	}
}

// TestPrismErrorInterceptorPassThrough ensures twirp.Error values
// returned from handlers pass through untouched.
func TestPrismErrorInterceptorPassThrough(t *testing.T) {
	orig := twirp.NewError(twirp.PermissionDenied, "no")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, orig
	}
	_, got := ErrorInterceptor(handler)(context.Background(), nil)
	var twerr twirp.Error
	if !errors.As(got, &twerr) {
		t.Fatalf("not a twirp error: %v", got)
	}
	if twerr.Code() != twirp.PermissionDenied {
		t.Fatalf("code = %v; want PermissionDenied", twerr.Code())
	}
}

// TestPrismErrorInterceptorSuccessPath: nil error passes through nil.
func TestPrismErrorInterceptorSuccessPath(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}
	resp, err := ErrorInterceptor(handler)(context.Background(), nil)
	if err != nil {
		t.Fatalf("got err %v; want nil", err)
	}
	if resp != "ok" {
		t.Fatalf("resp = %v; want ok", resp)
	}
}
