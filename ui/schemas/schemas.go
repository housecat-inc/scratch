// Package schemas holds schemas.gen.ts, an Effect Schema for each Go type the
// shared UI's scripts decode or send (ADR-0004), which ui/cmd/browserschemas
// lists. The output is committed, and go generate rebuilds it; CI fails when
// what it generates differs from what was committed. app/bundles generates it
// before it builds the bundles, which import it.
package schemas

