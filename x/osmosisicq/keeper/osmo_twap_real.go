//go:build never_osmosis_real

package keeper

// The real Osmosis integration is intentionally disabled. This stub prevents
// importing any Osmosis packages so that the module graph does not pull
// github.com/osmosis-labs/osmosis/v30 (which depends on ibc-go/v8).

// This file intentionally defines no symbols. The production integration is
// provided via other files behind different build tags.
