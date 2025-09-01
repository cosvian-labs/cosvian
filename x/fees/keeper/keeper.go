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
    case "dex_swap_native", "dex_native":
        return params.FeeTableUsd.DexNative.UsdAmount
    case "dex_swap_user", "dex_user":
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
	case "dex_swap_native", "dex_native":
		feeConfig = params.FeeTableUsd.DexNative
	case "dex_swap_user", "dex_user":
		feeConfig = params.FeeTableUsd.DexUser
	case "token_creation", "contract_deploy":
		feeConfig = params.FeeTableUsd.Deploy
	default:
		feeConfig = params.FeeTableUsd.NativeTransfer
	}

    // Perform distribution per split config, but resolve dynamic recipients from metadata
    amount := feeCoin.Amount

	treasuryAmount := feeConfig.Split.Treasury.MulInt(amount).TruncateInt()
	retailWalletAmount := feeConfig.Split.RetailWallet.MulInt(amount).TruncateInt()
	tokenDevAmount := feeConfig.Split.TokenDev.MulInt(amount).TruncateInt()
	tokenCreatorAmount := feeConfig.Split.TokenCreator.MulInt(amount).TruncateInt()

	total := treasuryAmount.Add(retailWalletAmount).Add(tokenDevAmount).Add(tokenCreatorAmount)
	if total.GT(amount) {
		treasuryAmount = amount.Sub(retailWalletAmount).Sub(tokenDevAmount).Sub(tokenCreatorAmount)
	}

    // Resolve dynamic recipients from metadata, fallback to params
    var retailAddr, devAddr, creatorAddr sdk.AccAddress
    // POS retail wallet address from metadata: "retail_address"
    if v, ok := metadata["retail_address"].(string); ok && v != "" {
        if addr, err := k.addressCodec.StringToBytes(v); err == nil {
            retailAddr = addr
        }
    }
    // Token creator/dev dynamic addresses can be passed directly via metadata
    // Prefer explicit address keys to avoid cross-module dependency
    if v, ok := metadata["creator_address"].(string); ok && v != "" {
        if addr, err := k.addressCodec.StringToBytes(v); err == nil {
            creatorAddr = addr
        }
    }
    if v, ok := metadata["dev_address"].(string); ok && v != "" {
        if addr, err := k.addressCodec.StringToBytes(v); err == nil {
            devAddr = addr
        }
    }
    // If only one is provided, mirror to the other (dev == creator by spec)
    if creatorAddr == nil && devAddr != nil {
        creatorAddr = devAddr
    }
    if devAddr == nil && creatorAddr != nil {
        devAddr = creatorAddr
    }

    // Fallback to params if dynamic not provided
    if retailAddr == nil {
        if s := params.RetailWallet; s != "" {
            retailAddr, _ = k.addressCodec.StringToBytes(s)
        }
    }
    if devAddr == nil {
        if s := params.TokenDevWallet; s != "" {
            devAddr, _ = k.addressCodec.StringToBytes(s)
        }
    }
    if creatorAddr == nil {
        if s := params.TokenCreatorWallet; s != "" {
            creatorAddr, _ = k.addressCodec.StringToBytes(s)
        }
    }

    // Fallback invalid recipient shares to treasury
    remap := math.ZeroInt()
    if retailAddr == nil || len(retailAddr) == 0 {
        remap = remap.Add(retailWalletAmount)
        retailWalletAmount = math.ZeroInt()
    }
    if devAddr == nil || len(devAddr) == 0 {
        remap = remap.Add(tokenDevAmount)
        tokenDevAmount = math.ZeroInt()
    }
    if creatorAddr == nil || len(creatorAddr) == 0 {
        remap = remap.Add(tokenCreatorAmount)
        tokenCreatorAmount = math.ZeroInt()
    }
    if !remap.IsZero() {
        treasuryAmount = treasuryAmount.Add(remap)
    }

    // Send to treasury module
    if !treasuryAmount.IsZero() {
        treasuryCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, treasuryAmount))
        if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, "treasury", treasuryCoins); err != nil {
            return fmt.Errorf("failed to send fees to treasury: %w", err)
        }
    }

    // Send to retail wallet account if configured
    if !retailWalletAmount.IsZero() && retailAddr != nil && len(retailAddr) > 0 {
        retailCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, retailWalletAmount))
        if err := k.bankKeeper.SendCoins(ctx, sender, retailAddr, retailCoins); err != nil {
            return fmt.Errorf("failed to send fees to retail wallet: %w", err)
        }
    }

    // Send to token developer account if configured
    if !tokenDevAmount.IsZero() && devAddr != nil && len(devAddr) > 0 {
        devCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, tokenDevAmount))
        if err := k.bankKeeper.SendCoins(ctx, sender, devAddr, devCoins); err != nil {
            return fmt.Errorf("failed to send fees to token developer: %w", err)
        }
    }

    // Send to token creator account if configured
    if !tokenCreatorAmount.IsZero() && creatorAddr != nil && len(creatorAddr) > 0 {
        creatorCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, tokenCreatorAmount))
        if err := k.bankKeeper.SendCoins(ctx, sender, creatorAddr, creatorCoins); err != nil {
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

// DistributeFeeFromModule distributes a fee coin held by a module account according to configured params.
// moduleName is the source module account currently holding the coins (e.g., types.ModuleName).
// feeType selects the fee table entry, feeCoin is in BTO denom.
func (k Keeper) DistributeFeeFromModule(ctx sdk.Context, moduleName, feeType string, feeCoin sdk.Coin, metadata map[string]interface{}) error {
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
    case "dex_swap_native", "dex_native":
        feeConfig = params.FeeTableUsd.DexNative
    case "dex_swap_user", "dex_user":
        feeConfig = params.FeeTableUsd.DexUser
    case "token_creation", "contract_deploy", "deploy":
        feeConfig = params.FeeTableUsd.Deploy
    default:
        feeConfig = params.FeeTableUsd.NativeTransfer
    }

    amount := feeCoin.Amount

    treasuryAmount := feeConfig.Split.Treasury.MulInt(amount).TruncateInt()
    retailWalletAmount := feeConfig.Split.RetailWallet.MulInt(amount).TruncateInt()
    tokenDevAmount := feeConfig.Split.TokenDev.MulInt(amount).TruncateInt()
    tokenCreatorAmount := feeConfig.Split.TokenCreator.MulInt(amount).TruncateInt()

    total := treasuryAmount.Add(retailWalletAmount).Add(tokenDevAmount).Add(tokenCreatorAmount)
    if total.GT(amount) {
        treasuryAmount = amount.Sub(retailWalletAmount).Sub(tokenDevAmount).Sub(tokenCreatorAmount)
    }

    var retailAddr, devAddr, creatorAddr sdk.AccAddress
    if v, ok := metadata["retail_address"].(string); ok && v != "" {
        if addr, err := k.addressCodec.StringToBytes(v); err == nil {
            retailAddr = addr
        }
    }
    if v, ok := metadata["creator_address"].(string); ok && v != "" {
        if addr, err := k.addressCodec.StringToBytes(v); err == nil {
            creatorAddr = addr
        }
    }
    if v, ok := metadata["dev_address"].(string); ok && v != "" {
        if addr, err := k.addressCodec.StringToBytes(v); err == nil {
            devAddr = addr
        }
    }
    if creatorAddr == nil && devAddr != nil {
        creatorAddr = devAddr
    }
    if devAddr == nil && creatorAddr != nil {
        devAddr = creatorAddr
    }

    if retailAddr == nil {
        if s := params.RetailWallet; s != "" {
            retailAddr, _ = k.addressCodec.StringToBytes(s)
        }
    }
    if devAddr == nil {
        if s := params.TokenDevWallet; s != "" {
            devAddr, _ = k.addressCodec.StringToBytes(s)
        }
    }
    if creatorAddr == nil {
        if s := params.TokenCreatorWallet; s != "" {
            creatorAddr, _ = k.addressCodec.StringToBytes(s)
        }
    }

    remap := math.ZeroInt()
    if retailAddr == nil || len(retailAddr) == 0 {
        remap = remap.Add(retailWalletAmount)
        retailWalletAmount = math.ZeroInt()
    }
    if devAddr == nil || len(devAddr) == 0 {
        remap = remap.Add(tokenDevAmount)
        tokenDevAmount = math.ZeroInt()
    }
    if creatorAddr == nil || len(creatorAddr) == 0 {
        remap = remap.Add(tokenCreatorAmount)
        tokenCreatorAmount = math.ZeroInt()
    }
    if !remap.IsZero() {
        treasuryAmount = treasuryAmount.Add(remap)
    }

    if !treasuryAmount.IsZero() {
        treasuryCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, treasuryAmount))
        if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, moduleName, "treasury", treasuryCoins); err != nil {
            return fmt.Errorf("failed to send fees to treasury: %w", err)
        }
    }
    if !retailWalletAmount.IsZero() && retailAddr != nil && len(retailAddr) > 0 {
        retailCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, retailWalletAmount))
        if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, moduleName, retailAddr, retailCoins); err != nil {
            return fmt.Errorf("failed to send fees to retail wallet: %w", err)
        }
    }
    if !tokenDevAmount.IsZero() && devAddr != nil && len(devAddr) > 0 {
        devCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, tokenDevAmount))
        if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, moduleName, devAddr, devCoins); err != nil {
            return fmt.Errorf("failed to send fees to token developer: %w", err)
        }
    }
    if !tokenCreatorAmount.IsZero() && creatorAddr != nil && len(creatorAddr) > 0 {
        creatorCoins := sdk.NewCoins(sdk.NewCoin(feeCoin.Denom, tokenCreatorAmount))
        if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, moduleName, creatorAddr, creatorCoins); err != nil {
            return fmt.Errorf("failed to send fees to token creator: %w", err)
        }
    }

    ctx.EventManager().EmitEvent(
        sdk.NewEvent("FeeCharged",
            sdk.NewAttribute("module", moduleName),
            sdk.NewAttribute("fee_type", feeType),
            sdk.NewAttribute("total_fee", feeCoin.String()),
        ),
    )

    return nil
}
