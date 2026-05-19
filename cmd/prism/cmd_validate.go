package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

// validateCommand returns the `prism validate` subcommand.
func validateCommand() *cli.Command {
	return &cli.Command{
		Name:      "validate",
		Usage:     "Validate a Prism spec file (or stdin) against the embedded JSON Schema and semantic rules",
		ArgsUsage: "[spec-file]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Emit a JSON error envelope instead of pretty text",
			},
		},
		Action: runValidate,
	}
}

// runValidate is the validate Action. Returns cli.Exit so the App.Run
// caller (and tests) see a meaningful exit code without process kill.
func runValidate(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	jsonOut := cmd.Bool("json")

	src, srcName, err := openSpec(args)
	if err != nil {
		return cli.Exit(err.Error(), 2)
	}
	defer src.Close()
	body, err := io.ReadAll(src)
	if err != nil {
		return cli.Exit(fmt.Sprintf("read %s: %v", srcName, err), 2)
	}

	// Strict-decode into the typed Spec; this catches unknown-field shape
	// errors that JSON Schema would also catch, but with a Go decode error.
	typedSpec, decodeErr := spec.DecodeBytes(body)
	if decodeErr != nil {
		ae := prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", decodeErr),
			map[string]any{"Schema": "(decode failed)"})
		return reportAndExit(cmd, jsonOut, []*prismerrors.AppError{ae}, srcName)
	}

	// Shape (JSON Schema) validation runs against a generic any tree
	// because the JSON Schema engine wants map[string]any input.
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return cli.Exit(fmt.Sprintf("re-parse %s for shape validation: %v", srcName, err), 2)
	}
	shape, err := validate.NewShapeValidator()
	if err != nil {
		return cli.Exit(fmt.Sprintf("init shape validator: %v", err), 2)
	}
	shapeErrs := shape.Validate(raw)
	if len(shapeErrs) > 0 {
		appErrs := make([]*prismerrors.AppError, 0, len(shapeErrs))
		for _, se := range shapeErrs {
			appErrs = append(appErrs, prismerrors.New(
				"PRISM_SPEC_009",
				fmt.Sprintf("Shape violation at %s: %s", se.InstanceLocation, se.Message),
				map[string]any{
					"Schema":           "shape",
					"Instance":         se.InstanceLocation,
					"KeywordViolation": se.KeywordLocation,
					"ViolationMessage": se.Message,
				},
			))
		}
		return reportAndExit(cmd, jsonOut, appErrs, srcName)
	}

	// Semantic validation runs against the typed spec.
	sem := validate.NewDefaultSemanticValidator()
	semErrs := sem.Validate(typedSpec, buildLookup(typedSpec))
	if len(semErrs) > 0 {
		return reportAndExit(cmd, jsonOut, semErrs, srcName)
	}

	// Success.
	if jsonOut {
		fmt.Fprintln(cmd.Writer, `{"format_version":"1.0","data":{"valid":true}}`)
	} else {
		fmt.Fprintln(cmd.Writer, "valid")
	}
	return nil
}

// openSpec resolves the input source: explicit file path, or stdin.
func openSpec(args []string) (io.ReadCloser, string, error) {
	switch {
	case len(args) == 0:
		return io.NopCloser(os.Stdin), "<stdin>", nil
	case len(args) == 1:
		f, err := os.Open(args[0])
		if err != nil {
			return nil, args[0], fmt.Errorf("open %s: %w", args[0], err)
		}
		return f, args[0], nil
	default:
		return nil, "", fmt.Errorf("expected at most one spec file argument, got %d", len(args))
	}
}

// buildLookup gives semantic rules a SchemaLookup driven by the spec's
// own data binding.
//
//   - Inline values / fields  -> StaticLookup populated from `data.values`
//     and `data.fields`, mirroring the P01 path.
//   - data.source             -> PulseLookup wired through resolve.New(nil)
//     against the on-disk file system. P02 makes the field-existence /
//     scale-compat rules fire against real schemas.
//   - Mixed (some inline, some source) -> CompositeLookup tries
//     PulseLookup first, then falls back to StaticLookup.
func buildLookup(s *spec.Spec) validate.SchemaLookup {
	staticLookup := validate.NewStaticLookup()
	pulseLookup := validate.NewPulseLookup(resolve.New(nil), afero.NewOsFs())
	usedPulse := false

	registerStatic := func(name string, ds *spec.Data) {
		if ds == nil {
			return
		}
		shim := &validate.PulseSchemaShim{Name: name}
		if len(ds.Values) > 0 {
			seen := map[string]bool{}
			for _, row := range ds.Values {
				for k, v := range row {
					if seen[k] {
						continue
					}
					seen[k] = true
					shim.Fields = append(shim.Fields, validate.FieldShim{
						Name: k, Type: inferMeasureType(v),
					})
				}
			}
		}
		for _, f := range ds.Fields {
			shim.Fields = append(shim.Fields, validate.FieldShim{
				Name: f.Name, Type: pulseStorageToMeasure(f.Type),
			})
		}
		if len(shim.Fields) == 0 {
			return
		}
		staticLookup.Register(name, shim)
	}

	registerPulse := func(name string, ds *spec.Data) {
		if ds == nil || ds.Source == "" {
			return
		}
		// Bind under both `data.name` (when present) and the source
		// basename so field-exists rules can find the schema regardless
		// of which key the spec used to address it.
		if name != "" {
			pulseLookup.Register(name, ds.Source)
			usedPulse = true
		}
		base := strings.TrimSuffix(filepath.Base(ds.Source), filepath.Ext(ds.Source))
		if base != "" && base != name {
			pulseLookup.Register(base, ds.Source)
			usedPulse = true
		}
		// Also bind the literal source string so semantic rules that
		// fall back to data.source see the same schema.
		pulseLookup.Register(ds.Source, ds.Source)
		usedPulse = true
	}

	walk := func(name string, ds *spec.Data) {
		registerStatic(name, ds)
		registerPulse(name, ds)
	}

	if s != nil {
		if s.Data != nil {
			walk(s.Data.Name, s.Data)
		}
		for name, ds := range s.Datasets {
			walk(name, ds)
		}
	}

	if !usedPulse {
		return staticLookup
	}
	return validate.NewCompositeLookup(pulseLookup, staticLookup)
}

// inferMeasureType maps a Go scalar value to a Prism measure-type bucket.
func inferMeasureType(v any) string {
	switch v.(type) {
	case float64, float32, int, int64, int32:
		return "quantitative"
	case bool:
		return "nominal"
	case string:
		return "nominal"
	default:
		return ""
	}
}

// pulseStorageToMeasure folds the FieldSpec.Type tokens (int/float/string/...)
// down to a measure type bucket. The full Pulse-storage type stays in
// the FieldSpec for downstream consumers.
func pulseStorageToMeasure(storage string) string {
	switch strings.ToLower(storage) {
	case "int", "int8", "int16", "int32", "int64", "float", "float32", "float64":
		return "quantitative"
	case "date", "datetime", "duration":
		return "temporal"
	default:
		return "nominal"
	}
}

// reportAndExit prints the error block and returns a cli.Exit so the
// App.Run caller gets exit code 1.
func reportAndExit(cmd *cli.Command, jsonOut bool, errs []*prismerrors.AppError, srcName string) error {
	if jsonOut {
		env := struct {
			FormatVersion string                  `json:"format_version"`
			Data          map[string]any          `json:"data,omitempty"`
			Errors        []*prismerrors.AppError `json:"errors"`
			Warnings      []*prismerrors.AppError `json:"warnings,omitempty"`
		}{
			FormatVersion: "1.0",
			Data:          map[string]any{"valid": false, "source": srcName},
			Errors:        errs,
		}
		enc := json.NewEncoder(cmd.Writer)
		enc.SetIndent("", "  ")
		_ = enc.Encode(env)
		return cli.Exit("", 1)
	}
	fmt.Fprintf(cmd.Writer, "invalid: %s\n", srcName)
	for _, e := range errs {
		fmt.Fprintf(cmd.Writer, "\nERROR %s: %s\n", e.Code, e.Message)
		if len(e.Fixups) > 0 {
			fmt.Fprintln(cmd.Writer, "Fixups:")
			for _, fx := range e.Fixups {
				fmt.Fprintf(cmd.Writer, "  - %s\n", fx)
			}
		}
		if len(e.SeeAlso) > 0 {
			fmt.Fprintf(cmd.Writer, "See also: %s\n", strings.Join(e.SeeAlso, ", "))
		}
	}
	fmt.Fprintf(cmd.Writer, "\nRun `prism errors lookup <code>` for more detail.\n")
	return cli.Exit("", 1)
}
