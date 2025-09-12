//go:build icq_async

package app

import (
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
)

// For controller-only integration, we don't host async-icq on this chain.
// Leave these hooks as no-ops under the tag as well.
func maybeRegisterICQStores(_ *App) error { return nil }
func registerICQAsync(_ *App, _ *porttypes.Router) error { return nil }
