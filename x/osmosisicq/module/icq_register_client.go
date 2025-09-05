//go:build icq_register

package osmosisicq

import (
	"fmt"
	"sync"

	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"bitora/x/osmosisicq/types"
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

type ICQClientOut struct {
    depinject.Out
    ICQClient types.ICQClient
}

// ProvideICQRegisterClient exposes a minimal ICQ client via depinject when built with -tags icq_register.
func ProvideICQRegisterClient() ICQClientOut { return ICQClientOut{ICQClient: &regOnlyClient{}} }

func init() {
    // Register the provider under this module's config when icq_register tag is enabled.
    appconfig.Register(&types.Module{}, appconfig.Provide(ProvideICQRegisterClient))
}
