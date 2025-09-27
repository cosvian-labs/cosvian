package osmosisicq

import (
	"fmt"

	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	sdk "github.com/cosmos/cosmos-sdk/types"

	icqctrlkeeper "bitora/x/icqcontroller/keeper"
	types "bitora/x/osmosisicq/types"
)

// Register a provider that adds the async-icq controller keeper to the container and
// supplies an ICQClient to osmosisicq when icq_async tag is enabled.
func init() {
	appconfig.Register(
		&types.Module{},
		appconfig.Provide(ProvideICQAsyncClient),
	)
}

type asyncInputs struct {
	depinject.In

	IcqCtrlKeeper *icqctrlkeeper.Keeper `optional:"true"`
}

type asyncOutputs struct {
	depinject.Out

	// Supply an ICQClient to osmosisicq module wiring.
	ICQClient types.ICQClient
}

func ProvideICQAsyncClient(in asyncInputs) (asyncOutputs, error) {
	// When the controller keeper isn't wired yet (e.g., during unit tests), fall back to nil
	// which results in intent events without actual sends.
	return asyncOutputs{ICQClient: icqClientAdapter{ctrl: in.IcqCtrlKeeper}}, nil
}

// icqClientAdapter implements types.ICQClient on top of async-icq controller keeper.
type icqClientAdapter struct{ ctrl *icqctrlkeeper.Keeper }

func (a icqClientAdapter) RegisterKVQuery(ctx sdk.Context, connectionID, store string, key []byte) (string, error) {
	ctrl := a.ctrl
	if ctrl == nil {
		return "", types.ErrNoActiveChannel
	}
	// Try sending over the controller module; if no channel yet, emit intent and return.
	if _, ok := ctrl.GetChannelForConnection(ctx, connectionID); !ok {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("icq_register_intent",
				sdk.NewAttribute("connection_id", connectionID),
				sdk.NewAttribute("store", store),
				sdk.NewAttribute("key_len", fmt.Sprintf("%d", len(key))),
				sdk.NewAttribute("note", "no_active_channel"),
			),
		)
		return "", types.ErrNoActiveChannel
	}
	qid, err := ctrl.SendKVQuery(ctx, connectionID, store, key)
	if err != nil {
		return "", err
	}
	return qid, nil
}
