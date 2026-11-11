// Housecat addition to the vendored templui utils: a TwMerge configured with
// @klinking/squircle's conflict rules, variable shadows and Button's value
// utilities.
package utils

import (
	"regexp"
	"strings"
	"sync"

	twmerge "github.com/Oudwins/tailwind-merge-go/pkg/twmerge"
)

// squircle-* shadows rounded-*, so without these rules TwMerge sees two
// unrelated groups and keeps both -- "rounded-lg squircle-md" would emit a
// border-radius that the squircle utility then overrides only where
// corner-shape is supported.
//
// Ported from squircleMergeConfig in @klinking/squircle v1.2.0, which the
// lockfile installs at ui/node_modules/@klinking/squircle/dist/tailwind/index.mjs.
// Every suffix has its own group and a side conflicts only with its own two
// corners. Pills have separate shape, side and knob groups, so a shape can
// replace its corners without discarding the parameters that paint it.
var (
	// Each side maps to the two corners it covers, so a side conflicts with
	// exactly the corners it would overwrite.
	squircleSideCorners = map[string][]string{
		"b": {"br", "bl"},
		"e": {"se", "ee"},
		"l": {"tl", "bl"},
		"r": {"tr", "br"},
		"s": {"ss", "es"},
		"t": {"tl", "tr"},
	}
	squircleCorners = []string{"bl", "br", "ee", "es", "se", "ss", "tl", "tr"}
	squircleSides   = []string{"b", "e", "l", "r", "s", "t"}
)

// squircleSuffixes lists "" (the all-corners utility) followed by every side
// and corner, matching upstream's ALL_SUFFIXES.
func squircleSuffixes() []string {
	out := []string{""}
	out = append(out, squircleSides...)
	return append(out, squircleCorners...)
}

func squircleGroup(suffix string) string {
	if suffix == "" {
		return "squircle"
	}
	return "squircle-" + suffix
}

func roundedGroup(suffix string) string {
	if suffix == "" {
		return "rounded"
	}
	return "rounded-" + suffix
}

func squircleClassPart() twmerge.ClassPart {
	radius := func(group string) []twmerge.ClassGroupValidator {
		return []twmerge.ClassGroupValidator{{Fn: twmerge.IsAny, ClassGroupId: group}}
	}

	next := map[string]twmerge.ClassPart{
		"amt": {
			Validators:   radius("squircle-amt"),
			ClassGroupId: "squircle-amt",
		},
		"full": {ClassGroupId: "squircle"},
		"none": {ClassGroupId: "squircle"},
		"pill": squirclePillClassPart(),
	}
	for _, suffix := range squircleSuffixes() {
		if suffix == "" {
			continue
		}
		group := squircleGroup(suffix)
		next[suffix] = twmerge.ClassPart{
			NextPart: map[string]twmerge.ClassPart{
				"full": {ClassGroupId: group},
				"none": {ClassGroupId: group},
			},
			Validators:   radius(group),
			ClassGroupId: group,
		}
	}
	return twmerge.ClassPart{NextPart: next, Validators: []twmerge.ClassGroupValidator{{
		Fn:           func(value string) bool { return value != "pill" && !strings.HasPrefix(value, "pill-") },
		ClassGroupId: "squircle",
	}}}
}

func squirclePillClassPart() twmerge.ClassPart {
	value := func(group string) []twmerge.ClassGroupValidator {
		return []twmerge.ClassGroupValidator{{Fn: twmerge.IsAny, ClassGroupId: group}}
	}
	return twmerge.ClassPart{
		NextPart: map[string]twmerge.ClassPart{
			"arc":  {NextPart: map[string]twmerge.ClassPart{"setback": {Validators: value("squircle-pill-arc-setback")}}},
			"b":    {ClassGroupId: "squircle-pill-side"},
			"e":    {ClassGroupId: "squircle-pill-side"},
			"edge": {NextPart: map[string]twmerge.ClassPart{"setback": {Validators: value("squircle-pill-edge-setback")}}},
			"g2":   {ClassGroupId: "squircle-pill-continuity"},
			"g3":   {ClassGroupId: "squircle-pill-continuity"},
			"l":    {ClassGroupId: "squircle-pill-side"},
			"max":  {Validators: value("squircle-pill-max")},
			"r":    {ClassGroupId: "squircle-pill-side"},
			"s":    {ClassGroupId: "squircle-pill-side"},
			"t":    {ClassGroupId: "squircle-pill-side"},
		},
		ClassGroupId: "squircle-pill",
	}
}

func squircleConfig() *twmerge.TwMergeConfig {
	config := twmerge.MakeDefaultConfig()
	config.ClassGroups.NextPart["squircle"] = squircleClassPart()

	suffixes := squircleSuffixes()

	// The all-corners utility overwrites every other squircle and every
	// rounded; a bare rounded overwrites every squircle.
	var all []string
	for _, suffix := range suffixes[1:] {
		all = append(all, squircleGroup(suffix))
	}
	for _, suffix := range suffixes {
		all = append(all, roundedGroup(suffix))
	}
	all = append(all, "squircle-pill", "squircle-pill-side")
	config.ConflictingClassGroups["squircle"] = all

	var everySquircle []string
	for _, suffix := range suffixes {
		everySquircle = append(everySquircle, squircleGroup(suffix))
	}
	config.ConflictingClassGroups["rounded"] = append(config.ConflictingClassGroups["rounded"], everySquircle...)
	config.ConflictingClassGroups["rounded"] = append(config.ConflictingClassGroups["rounded"], "squircle-pill", "squircle-pill-side")
	pillConflicts := append([]string{}, everySquircle...)
	for _, suffix := range suffixes {
		pillConflicts = append(pillConflicts, roundedGroup(suffix))
	}
	config.ConflictingClassGroups["squircle-pill"] = pillConflicts

	for _, side := range squircleSides {
		corners := squircleSideCorners[side]

		var sideConflicts []string
		for _, corner := range corners {
			sideConflicts = append(sideConflicts, squircleGroup(corner))
		}
		sideConflicts = append(sideConflicts, roundedGroup(side))
		for _, corner := range corners {
			sideConflicts = append(sideConflicts, roundedGroup(corner))
		}
		config.ConflictingClassGroups[squircleGroup(side)] = sideConflicts

		roundedSideConflicts := []string{squircleGroup(side)}
		for _, corner := range corners {
			roundedSideConflicts = append(roundedSideConflicts, squircleGroup(corner))
		}
		config.ConflictingClassGroups[roundedGroup(side)] = append(
			config.ConflictingClassGroups[roundedGroup(side)], roundedSideConflicts...)
	}

	for _, corner := range squircleCorners {
		config.ConflictingClassGroups[squircleGroup(corner)] = []string{roundedGroup(corner)}
		config.ConflictingClassGroups[roundedGroup(corner)] = append(
			config.ConflictingClassGroups[roundedGroup(corner)], squircleGroup(corner))
	}

	return config
}

// variableValue matches a utility value that is a CSS variable, in Tailwind
// v4's parenthesised form or a bracketed var() -- fallback included, since
// Tailwind infers no type for any bracketed value that starts with var(.
var variableValue = regexp.MustCompile(`^(\(--[\w-]+\)|\[var\(--[^\]]*\)\])$`)

func isVariableValue(val string) bool {
	return variableValue.MatchString(val)
}

// A shadow drawn from a variable -- shadow-(--button-shadow), as Button paints
// its own -- is a shadow, not a shadow colour. The default config knows no
// variable form: its shadow validators want a t-shirt size or a literal shadow,
// and a value they refuse falls through to shadow-color's catch-all, where a
// caller's shadow-md would sit beside the variable shadow instead of replacing
// it. The validator goes ahead of that catch-all. Colour groups need nothing:
// their catch-all already takes a variable as a colour, which is what it is.
func variableShadowConfig(config *twmerge.TwMergeConfig) *twmerge.TwMergeConfig {
	shadow := config.ClassGroups.NextPart["shadow"]
	shadow.Validators = append(
		[]twmerge.ClassGroupValidator{{Fn: isVariableValue, ClassGroupId: "shadow"}},
		shadow.Validators...,
	)
	config.ClassGroups.NextPart["shadow"] = shadow
	return config
}

// Button's value utilities (ui/css/button.css) are two class groups,
// button-size and button-variant, so a later one replaces an earlier one the
// way any utility replaces another of its group: a bare button-variant-* in a
// Button's Class replaces the one its Variant renders. TwMerge keys a group by
// its variant prefixes as well, so one under docked: sits beside the bare one
// instead. The port splits a class on "-" and walks NextPart, so each group
// hangs off "button" with IsAny taking the value, as the squircle groups do.
func buttonConfig(config *twmerge.TwMergeConfig) *twmerge.TwMergeConfig {
	group := func(id string) twmerge.ClassPart {
		return twmerge.ClassPart{Validators: []twmerge.ClassGroupValidator{{Fn: twmerge.IsAny, ClassGroupId: id}}}
	}
	config.ClassGroups.NextPart["button"] = twmerge.ClassPart{
		NextPart: map[string]twmerge.ClassPart{
			"size":    group("button-size"),
			"variant": group("button-variant"),
		},
	}
	return config
}

// cap-trim and truncate-inline (ui/css/text_trim.css). cap-trim sets only text-box,
// which Tailwind has no utility for, so it is a group of its own and conflicts
// with nothing. truncate-inline sets overflow-x, text-overflow and white-space, so
// it trades places with every utility for those -- truncate is one of
// text-overflow's -- whichever comes later.
func textTrimConfig(config *twmerge.TwMergeConfig) *twmerge.TwMergeConfig {
	config.ClassGroups.NextPart["cap"] = twmerge.ClassPart{
		NextPart: map[string]twmerge.ClassPart{"trim": {ClassGroupId: "cap-trim"}},
	}

	truncate := config.ClassGroups.NextPart["truncate"]
	truncate.NextPart = map[string]twmerge.ClassPart{"inline": {ClassGroupId: "truncate-inline"}}
	config.ClassGroups.NextPart["truncate"] = truncate

	axes := []string{"overflow", "overflow-x", "text-overflow", "whitespace"}
	config.ConflictingClassGroups["truncate-inline"] = axes
	for _, axis := range axes {
		config.ConflictingClassGroups[axis] = append(config.ConflictingClassGroups[axis], "truncate-inline")
	}
	return config
}

func newTwMerge() twmerge.TwMergeFn {
	merge := twmerge.CreateTwMerge(textTrimConfig(typeConfig(variableShadowConfig(buttonConfig(squircleConfig())))), nil)
	var mu sync.Mutex
	return func(classes ...string) string {
		mu.Lock()
		defer mu.Unlock()
		return merge(classes...)
	}
}

var twMerge = newTwMerge()
