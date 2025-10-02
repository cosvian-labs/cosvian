//go:build osmosis_removed

package osmosisicq

import (
	"fmt"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosvian/x/osmosisicq/types"
)

// regOnlyClient is a minimal ICQ client that only registers queries and returns synthetic IDs.
// This enables real KV query registration flows in Keeper without pulling external ICQ deps.
type regOnlyClient struct {
	mu  sync.Mutex
	seq int
}

var _ types.ICQClient = (*regOnlyClient)(nil)

func (c *regOnlyClient) RegisterKVQuery(_ sdk.Context, _ string, _ string, _ []byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	return fmt.Sprintf("icq-%d", c.seq), nil
}

// Note: We no longer self-register this provider via appconfig.Register to avoid
// overriding the main module ProvideModule registration. Instead, the module's
// ProvideModule will instantiate this minimal client when no external ICQ client
// implementation is supplied (see depinject.go).
