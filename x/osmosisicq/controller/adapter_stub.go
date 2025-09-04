package controller

import (
	"fmt"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"bitora/x/osmosisicq/keeper"
	"bitora/x/osmosisicq/types"
)

// Ensure Adapter implements the ICQClient interface.
var _ types.ICQClient = (*Adapter)(nil)

// Adapter is a simple in-process ICQ client stub you can use to
// - register queries (returns synthetic IDs), and
// - deliver KV results into the osmosisicq keeper via ReceiveKVResult.
//
// This is useful for early E2E and local relayer prototypes. Replace with a
// real ICQ controller integration when ready.
type Adapter struct {
    k   keeper.Keeper
    mu  sync.Mutex
    seq int
    q   map[string]struct{ store string; key []byte }
}

func NewAdapter(k keeper.Keeper) *Adapter {
    return &Adapter{ k: k, q: make(map[string]struct{ store string; key []byte }) }
}

// RegisterKVQuery records the target and returns a synthetic query ID.
func (a *Adapter) RegisterKVQuery(ctx sdk.Context, connectionID, store string, key []byte) (string, error) {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.seq++
    id := fmt.Sprintf("qid-%d", a.seq)
    a.q[id] = struct{ store string; key []byte }{ store: store, key: key }
    return id, nil
}

// ReceiveKVResult can be called by an external controller when a KV value arrives.
// It will route to the keeper's OnKVResult for decoding and persistence.
func (a *Adapter) ReceiveKVResult(ctx sdk.Context, queryID string, value []byte) error {
    a.mu.Lock()
    target, ok := a.q[queryID]
    a.mu.Unlock()
    if !ok {
        return nil // unknown or already cleaned up; ignore
    }
    return a.k.OnKVResult(ctx, target.store, target.key, value)
}
 
