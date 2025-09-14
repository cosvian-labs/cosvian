package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	"bitora/x/icqcontroller/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	Port collections.Item[string]
	// ActiveChannel maps a connection-id to the open channel-id used for ICQ
	ActiveChannel collections.Map[string, string]

	// Pending correlates (channelID, sequence) -> JSON{"store","key_b64","connection"}
	Pending collections.Map[string, string]

	ibcKeeperFn func() *ibckeeper.Keeper
	// scopedKeeper (optional) allows this module to own the IBC port capability so
	// channel handshakes can succeed. When unset, genesis will skip capability ops.
	scopedKeeper *capabilitykeeper.ScopedKeeper

	// optional consumer for KV results (e.g., x/osmosisicq keeper)
	consumer types.KVResultConsumer
}

// testAckExtractor is a test-only seam for decoding acks without importing async-icq in tests.
// Set via SetTestAckExtractor from tests; left nil in normal builds.
var testAckExtractor func(ctx sdk.Context, acknowledgement []byte) ([]byte, error)

// SetTestAckExtractor allows tests to inject a custom ack decoder.
func SetTestAckExtractor(f func(ctx sdk.Context, acknowledgement []byte) ([]byte, error)) {
	testAckExtractor = f
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	ibcKeeperFn func() *ibckeeper.Keeper,
	scopedKeeper *capabilitykeeper.ScopedKeeper,
	// optional consumer, may be nil
	consumer types.KVResultConsumer,

) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,

	ibcKeeperFn:  ibcKeeperFn,
		scopedKeeper: scopedKeeper,
	consumer:     consumer,
	Port:          collections.NewItem(sb, types.PortKey, "port", collections.StringValue),
	Params:        collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
	ActiveChannel: collections.NewMap(sb, collections.NewPrefix("ac_icqcontroller"), "active_channel", collections.StringKey, collections.StringValue),
	Pending:       collections.NewMap(sb, collections.NewPrefix("pnd_icqcontroller"), "pending", collections.StringKey, collections.StringValue),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

// IBCKeeper returns the injected IBC keeper instance (if available).
func (k Keeper) IBCKeeper() *ibckeeper.Keeper { return k.ibcKeeperFn() }
// SetActiveChannel records the active channel-id for a given connection-id.
func (k Keeper) SetActiveChannel(ctx sdk.Context, connectionID, channelID string) error {
	return k.ActiveChannel.Set(ctx, connectionID, channelID)
}

// RemoveActiveChannel removes any mapping for the given connection-id.
func (k Keeper) RemoveActiveChannel(ctx sdk.Context, connectionID string) error {
	return k.ActiveChannel.Remove(ctx, connectionID)
}

// GetChannelForConnection returns the channel-id bound to the given connection-id, if any.
func (k Keeper) GetChannelForConnection(ctx sdk.Context, connectionID string) (string, bool) {
	ch, err := k.ActiveChannel.Get(ctx, connectionID)
	if err != nil {
		return "", false
	}
	return ch, true
}

// composite key helpers for Pending map
func pendingKey(channelID string, sequence uint64) string {
	return channelID + "/" + strconv.FormatUint(sequence, 10)
}

type pendingValue struct {
	Store      string `json:"store"`
	KeyB64     string `json:"key_b64"`
	Connection string `json:"connection"`
}

// SetPending stores correlation for (channel,sequence)
func (k Keeper) SetPending(ctx sdk.Context, channelID string, sequence uint64, store string, key []byte, connectionID string) error {
	pv := pendingValue{Store: store, KeyB64: base64.StdEncoding.EncodeToString(key), Connection: connectionID}
	b, _ := json.Marshal(pv)
	return k.Pending.Set(ctx, pendingKey(channelID, sequence), string(b))
}

// GetPending retrieves correlation info; returns ok=false if not found.
func (k Keeper) GetPending(ctx sdk.Context, channelID string, sequence uint64) (store string, key []byte, connectionID string, ok bool) {
	s, err := k.Pending.Get(ctx, pendingKey(channelID, sequence))
	if err != nil {
		return "", nil, "", false
	}
	var pv pendingValue
	if err := json.Unmarshal([]byte(s), &pv); err != nil {
		return "", nil, "", false
	}
	kb, err := base64.StdEncoding.DecodeString(pv.KeyB64)
	if err != nil {
		return pv.Store, nil, pv.Connection, true
	}
	return pv.Store, kb, pv.Connection, true
}

// RemovePending deletes correlation entry if present.
func (k Keeper) RemovePending(ctx sdk.Context, channelID string, sequence uint64) error {
	return k.Pending.Remove(ctx, pendingKey(channelID, sequence))
}

// HandlePacketAcknowledgement processes an IBC acknowledgement for a sent ICQ packet.
// It will parse the async-icq acknowledgement when built with icq_async; otherwise it no-ops.
func (k Keeper) HandlePacketAcknowledgement(ctx sdk.Context, portID, channelID string, sequence uint64, acknowledgement []byte) error {
	// Look up pending correlation first; if none, nothing to do.
	store, key, connectionID, ok := k.GetPending(ctx, channelID, sequence)
	if !ok {
		return nil
	}
	defer func() { _ = k.RemovePending(ctx, channelID, sequence) }()

	// Parse IBC ack wrapper for status and error string (safe in all builds)
	var ack channeltypes.Acknowledgement
	ackErrStr := ""
	ackSuccess := false
	if err := json.Unmarshal(acknowledgement, &ack); err == nil {
		ackSuccess = ack.Success()
		if !ackSuccess {
			ackErrStr = ack.GetError()
		}
	}

	// If no consumer wired, just drop after clearing.
	if k.consumer == nil {
		// Still emit an event about the ack outcome for observability
		if ackSuccess {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"icq_ack_success",
					sdk.NewAttribute("port", portID),
					sdk.NewAttribute("channel", channelID),
					sdk.NewAttribute("sequence", strconv.FormatUint(sequence, 10)),
					sdk.NewAttribute("connection", connectionID),
					sdk.NewAttribute("store", store),
					sdk.NewAttribute("key_b64", base64.StdEncoding.EncodeToString(key)),
				),
			)
		} else {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"icq_ack_error",
					sdk.NewAttribute("port", portID),
					sdk.NewAttribute("channel", channelID),
					sdk.NewAttribute("sequence", strconv.FormatUint(sequence, 10)),
					sdk.NewAttribute("connection", connectionID),
					sdk.NewAttribute("store", store),
					sdk.NewAttribute("key_b64", base64.StdEncoding.EncodeToString(key)),
					sdk.NewAttribute("error", ackErrStr),
				),
			)
		}
		return nil
	}

	// Parse success acknowledgement only under icq_async build tag by default.
	// Tests may inject a seam to override decoding without async-icq imports.
	var (
		value []byte
		err error
	)
	if testAckExtractor != nil {
		value, err = testAckExtractor(ctx, acknowledgement)
	} else {
		value, err = k.extractValueFromAck(ctx, acknowledgement)
	}
	if err != nil || len(value) == 0 {
		// emit error or empty-result event
		evType := "icq_ack_error"
		if err == nil && ackSuccess {
			// ack success but no value decoded
			evType = "icq_ack_success"
		} else if err == nil {
			// ack error but no extra err info
			ackErrStr = ackErrStr
		}
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				evType,
				sdk.NewAttribute("port", portID),
				sdk.NewAttribute("channel", channelID),
				sdk.NewAttribute("sequence", strconv.FormatUint(sequence, 10)),
				sdk.NewAttribute("connection", connectionID),
				sdk.NewAttribute("store", store),
				sdk.NewAttribute("key_b64", base64.StdEncoding.EncodeToString(key)),
				sdk.NewAttribute("error", ackErrStr),
			),
		)
		return err
	}

	// emit success then deliver to consumer
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"icq_ack_success",
			sdk.NewAttribute("port", portID),
			sdk.NewAttribute("channel", channelID),
			sdk.NewAttribute("sequence", strconv.FormatUint(sequence, 10)),
			sdk.NewAttribute("connection", connectionID),
			sdk.NewAttribute("store", store),
			sdk.NewAttribute("key_b64", base64.StdEncoding.EncodeToString(key)),
			sdk.NewAttribute("value_len", strconv.Itoa(len(value))),
		),
	)

	return k.consumer.OnKVResult(ctx, store, key, value)
}

// HandlePacketTimeout cleans up pending correlation on timeout and emits a lightweight event.
func (k Keeper) HandlePacketTimeout(ctx sdk.Context, portID, channelID string, sequence uint64) error {
	// remove mapping if present
	store, key, connectionID, _ := k.GetPending(ctx, channelID, sequence)
	_ = k.RemovePending(ctx, channelID, sequence)
	// emit timeout event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"icq_timeout",
			sdk.NewAttribute("port", portID),
			sdk.NewAttribute("channel", channelID),
			sdk.NewAttribute("sequence", strconv.FormatUint(sequence, 10)),
			sdk.NewAttribute("connection", connectionID),
			sdk.NewAttribute("store", store),
			sdk.NewAttribute("key_b64", base64.StdEncoding.EncodeToString(key)),
		),
	)
	return nil
}
