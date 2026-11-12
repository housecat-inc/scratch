package utils

import (
	twmerge "github.com/Oudwins/tailwind-merge-go/pkg/twmerge"
)

// type-<use>-<size>[-<weight>] sets every text axis at once
// (ui/css/typography.css): family, size, line-height, letter-spacing, weight,
// style and transform. A later type-* therefore replaces any earlier utility
// for one of those axes, and two type-* classes replace each other. A
// single-axis utility after a type-* survives: Tailwind emits the
// multi-property type-* rules ahead of the single-property ones, so
// "type-body-200 font-semibold" reads as body text with the weight overridden.
//
// type-bold and type-italic are <b> and <i> as classes: each claims one axis,
// so it trades places with the raw utility for that axis and is replaced by a
// later type-* the same way.
var typeConflicts = []string{"font-family", "font-size", "font-style", "font-weight", "leading", "text-transform", "tracking", "type-bold", "type-italic"}

func typeConfig(config *twmerge.TwMergeConfig) *twmerge.TwMergeConfig {
	config.ClassGroups.NextPart["type"] = twmerge.ClassPart{
		NextPart: map[string]twmerge.ClassPart{
			"bold":   {ClassGroupId: "type-bold"},
			"italic": {ClassGroupId: "type-italic"},
		},
		Validators: []twmerge.ClassGroupValidator{{Fn: twmerge.IsAny, ClassGroupId: "type"}},
	}
	config.ConflictingClassGroups["type"] = typeConflicts
	config.ConflictingClassGroups["type-bold"] = []string{"font-weight"}
	config.ConflictingClassGroups["type-italic"] = []string{"font-style"}
	config.ConflictingClassGroups["font-weight"] = append(config.ConflictingClassGroups["font-weight"], "type-bold")
	config.ConflictingClassGroups["font-style"] = append(config.ConflictingClassGroups["font-style"], "type-italic")
	return config
}
