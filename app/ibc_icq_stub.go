//go:build !icq_async

package app

import (
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
)

// maybeRegisterICQStores is a no-op by default. A build-tagged file can override
// this with real ICQ store registrations if needed.
func maybeRegisterICQStores(_ *App) error { return nil }

// registerICQAsync is a no-op by default. A build-tagged file can override this
// to add ICQ controller routes into the IBC router stack.
func registerICQAsync(_ *App, _ *porttypes.Router) error { return nil }
