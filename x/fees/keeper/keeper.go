package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	math "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"bitora/x/fees/types"
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

	oracleKeeper types.OracleKeeper
	bankKeeper   types.BankKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	oracleKeeper types.OracleKeeper,
	bankKeeper types.BankKeeper,
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

	oracleKeeper: oracleKeeper,
	bankKeeper:   bankKeeper,
		Params:       collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
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

// ConvertUSDToBTO converts a USD amount to BTO using the module's oracle adapter
func (k Keeper) ConvertUSDToBTO(ctx sdk.Context, usd math.LegacyDec) (math.LegacyDec, *types.PriceData, error) {
	oa := NewOracleAdapter(k, k.oracleKeeper)
	bto, pd, err := oa.ConvertUSDToBTO(ctx, usd)
	return bto, pd, err
}

// GetFeeByType returns the USD fee amount for a given fee type by reading module params
func (k Keeper) GetFeeByType(ctx sdk.Context, feeType string) math.LegacyDec {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return math.LegacyZeroDec()
	}
	switch feeType {
	case "pos_payment":
		return params.FeeTableUsd.PosPayment.UsdAmount
	case "token_interaction":
		return params.FeeTableUsd.TokenInteraction.UsdAmount
	case "native_transfer":
		return params.FeeTableUsd.NativeTransfer.UsdAmount
	case "dex_swap_native":
		return params.FeeTableUsd.DexNative.UsdAmount
	case "dex_swap_user":
		return params.FeeTableUsd.DexUser.UsdAmount
	case "token_creation", "contract_deploy":
		return params.FeeTableUsd.Deploy.UsdAmount
	default:
		return params.FeeTableUsd.NativeTransfer.UsdAmount
	}
}

// DistributeFee distributes a fee coin according to configured params for the feeType.
// sender is the account paying the fee, feeType selects the fee table entry, feeCoin is in BTO denom.
func (k Keeper) DistributeFee(ctx sdk.Context, sender sdk.AccAddress, feeType string, feeCoin sdk.Coin, metadata map[string]interface{}) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get params: %w", err)
	}

	var feeConfig types.FeeConfig
	switch feeType {
	case "pos_payment":
		feeConfig = params.FeeTableUsd.PosPayment
	case "token_interaction":
		feeConfig = params.FeeTableUsd.TokenInteraction
	case "native_transfer":
		feeConfig = params.FeeTableUsd.NativeTransfer
	case "dex_swap_native":
		feeConfig = params.FeeTableUsd.DexNative
	case "dex_swap_user":
		feeConfig = params.FeeTableUsd.DexUser
	case "token_creation", "contract_deploy":
		feeConfig = params.FeeTableUsd.Deploy
	default:
		feeConfig = params.FeeTableUsd.NativeTransfer
	}

	// Perform distribution similar to DeductFeeDecorator.distributeFees
	amount := feeCoin.Amount

	treasuryAmount := feeConfig.Split.Treasury.MulInt(amount).TruncateInt()
	retailWalletAmount := feeConfig.Split.RetailWallet.MulInt(amount).TruncateInt()
	tokenDevAmount := feeConfig.Split.TokenDev.MulInt(amount).TruncateInt()
	tokenCreatorAmount := feeConfig.Split.TokenCreator.MulInt(amount).TruncateInt()

	total := treasuryAmount.Add(retailWalletAmount).Add(tokenDevAmount).Add(tokenCreatorAmount)
	if total.GT(amount) {
		treasuryAmount = amount.Sub(retailWalletAmount).Sub(tokenDevAmount).Sub(tokenCreatorAmount)
	}

	// Send to treasury
	if !treasuryAmount.IsZero() {
		treasuryCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, treasuryAmount))
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, "treasury", treasuryCoins); err != nil {
			return fmt.Errorf("failed to send fees to treasury: %w", err)
		}
	}

	// Send to retail wallet (fallback to treasury for now)
	if !retailWalletAmount.IsZero() {
		retailCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, retailWalletAmount))
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, "treasury", retailCoins); err != nil {
			return fmt.Errorf("failed to send fees to retail wallet: %w", err)
		}
	}

	// Send to token developer (fallback to treasury)
	if !tokenDevAmount.IsZero() {
		devCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, tokenDevAmount))
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, "treasury", devCoins); err != nil {
			return fmt.Errorf("failed to send fees to token developer: %w", err)
		}
	}

	// Send to token creator (fallback to treasury)
	if !tokenCreatorAmount.IsZero() {
		creatorCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, tokenCreatorAmount))
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, "treasury", creatorCoins); err != nil {
			return fmt.Errorf("failed to send fees to token creator: %w", err)
		}
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("sender", sender.String()),
			sdk.NewAttribute("fee_type", feeType),
			sdk.NewAttribute("total_fee", feeCoin.String()),
			sdk.NewAttribute("treasury_fee", sdk.NewCoin(feeCoin.Denom, treasuryAmount).String()),
		),
	)

	return nil
}
