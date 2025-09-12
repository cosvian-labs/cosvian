//go:build icq_async

package osmosisicq

import (
	"fmt"

	"cosmossdk.io/core/address"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	icqctrlkeeper "bitora/x/icqcontroller/keeper"
	"bitora/x/osmosisicq/types"
)

// Register a provider that adds the async-icq controller keeper to the container and
// supplies an ICQClient to osmosisicq when icq_async tag is enabled.
func init() {
	appconfig.Register(
		&types.Module{},
		appconfig.Provide(provideICQAsyncClient),
	)
}

type asyncInputs struct {
	depinject.In

	Cdc          codec.Codec
	AddressCodec address.Codec
	IcqCtrlKeeper icqctrlkeeper.Keeper
}

type asyncOutputs struct {
	depinject.Out

	// Supply an ICQClient to osmosisicq module wiring.
	ICQClient types.ICQClient
}

func provideICQAsyncClient(in asyncInputs) (asyncOutputs, error) {
	return asyncOutputs{ICQClient: icqClientAdapter{ctrl: in.IcqCtrlKeeper}}, nil
}

// icqClientAdapter implements types.ICQClient on top of async-icq controller keeper.
type icqClientAdapter struct{ ctrl icqctrlkeeper.Keeper }

func (a icqClientAdapter) RegisterKVQuery(ctx sdk.Context, connectionID, store string, key []byte) (string, error) {
	// Try sending over the controller module; if no channel yet, emit intent and return.
	if _, ok := a.ctrl.GetChannelForConnection(ctx, connectionID); !ok {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("icq_register_intent",
				sdk.NewAttribute("connection_id", connectionID),
				sdk.NewAttribute("store", store),
				sdk.NewAttribute("key_len", fmt.Sprintf("%d", len(key))),
				sdk.NewAttribute("note", "no_active_channel"),
			),
		)
		return "", fmt.Errorf("no active icq channel for %s", connectionID)
	}
	qid, err := a.ctrl.SendKVQuery(ctx, connectionID, store, key)
	if err != nil {
		return "", err
	}
	return qid, nil
}
