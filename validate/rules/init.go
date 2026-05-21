package rules

import "github.com/frankbardon/prism/validate"

// Register the canonical PRISM_SPEC_001..009 rule set on the validate
// package's default registry. CLI / library callers can then build a
// fully-wired SemanticValidator via validate.NewDefaultSemanticValidator()
// without naming each rule.
func init() {
	validate.RegisterDefault(func() validate.SemanticRule { return FieldExists{} })
	validate.RegisterDefault(func() validate.SemanticRule { return AggCompat{} })
	validate.RegisterDefault(func() validate.SemanticRule { return ChannelForMark{} })
	validate.RegisterDefault(func() validate.SemanticRule { return SelectionRef{} })
	validate.RegisterDefault(func() validate.SemanticRule { return DatasetRef{} })
	validate.RegisterDefault(func() validate.SemanticRule { return ExpressionParses{} })
	validate.RegisterDefault(func() validate.SemanticRule { return ScaleTypeCompat{} })
	validate.RegisterDefault(func() validate.SemanticRule { return PieDonutEncoding{} })
	validate.RegisterDefault(func() validate.SemanticRule { return SchemaRef{} })
	validate.RegisterDefault(func() validate.SemanticRule { return LogScalePositiveDomain{} })
	validate.RegisterDefault(func() validate.SemanticRule { return FormatStringValid{} })
	validate.RegisterDefault(func() validate.SemanticRule { return ResolveScaleCompat{} })
	validate.RegisterDefault(func() validate.SemanticRule { return RepeatSubstitution{} })
	validate.RegisterDefault(func() validate.SemanticRule { return CompositeMarkEncoding{} })
	validate.RegisterDefault(func() validate.SemanticRule { return ImageURLAllowed{} })
	validate.RegisterDefault(func() validate.SemanticRule { return PathDNonEmpty{} })
	validate.RegisterDefault(func() validate.SemanticRule { return SankeyChannels{} })
	validate.RegisterDefault(func() validate.SemanticRule { return SelectionEncodingChannel{} })
	validate.RegisterDefault(func() validate.SemanticRule { return SelectionIntervalChannel{} })
	validate.RegisterDefault(func() validate.SemanticRule { return GeoProjection{} })
	validate.RegisterDefault(func() validate.SemanticRule { return AnimationEasingKnown{} })
	validate.RegisterDefault(func() validate.SemanticRule { return AnimationKeyPresent{} })
	validate.RegisterDefault(func() validate.SemanticRule { return AnimationKeyUnique{} })
}
