package keeper

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"cosvian/x/treasury/types"
)

// LockedReward represents a time-locked reward for retail wallet POS payments
type LockedReward struct {
	Recipient  string    `json:"recipient"`
	Amount     sdk.Coin  `json:"amount"`
	LockTime   time.Time `json:"lock_time"`
	UnlockTime time.Time `json:"unlock_time"`
	FeeType    string    `json:"fee_type"`
	IsUnlocked bool      `json:"is_unlocked"`
}

// LockRetailWalletReward locks 50% of POS payment fee for retail wallet (6 month lock)
func (k Keeper) LockRetailWalletReward(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coin) error {
	unlockTime := ctx.BlockTime().Add(time.Hour * 24 * 30 * 6) // 6 months

	reward := LockedReward{
		Recipient:  recipient.String(),
		Amount:     amount,
		LockTime:   ctx.BlockTime(),
		UnlockTime: unlockTime,
		FeeType:    "pos_payment",
		IsUnlocked: false,
	}

	// Store the locked reward
	err := k.storeLockedReward(ctx, reward)
	if err != nil {
		return sdkerrors.ErrInvalidRequest.Wrapf("failed to store locked reward: %v", err)
	}

	// Transfer amount to locked_rewards module account
	lockedRewardsAddr := k.GetModuleAddress("locked_rewards")
	err = k.bankKeeper.SendCoins(ctx, recipient, lockedRewardsAddr, sdk.NewCoins(amount))
	if err != nil {
		return sdkerrors.ErrInsufficientFunds.Wrapf("failed to lock reward: %v", err)
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"retail_reward_locked",
			sdk.NewAttribute("recipient", recipient.String()),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("unlock_time", unlockTime.Format(time.RFC3339)),
		),
	)

	return nil
}

// ProcessUnlockedRewards checks and processes all rewards that can be unlocked
func (k Keeper) ProcessUnlockedRewards(ctx sdk.Context) error {
	// Get all locked rewards
	lockedRewards, err := k.getAllLockedRewards(ctx)
	if err != nil {
		return err
	}

	lockedRewardsAddr := k.GetModuleAddress("locked_rewards")
	currentTime := ctx.BlockTime()

	for _, reward := range lockedRewards {
		// Skip if already unlocked or not yet time to unlock
		if reward.IsUnlocked || currentTime.Before(reward.UnlockTime) {
			continue
		}

		// Unlock the reward
		recipientAddr, err := sdk.AccAddressFromBech32(reward.Recipient)
		if err != nil {
			continue // Skip invalid addresses
		}

		// Send coins from locked module account to recipient
		err = k.bankKeeper.SendCoins(ctx, lockedRewardsAddr, recipientAddr, sdk.NewCoins(reward.Amount))
		if err != nil {
			continue // Skip if insufficient funds (shouldn't happen)
		}

		// Mark as unlocked
		reward.IsUnlocked = true
		err = k.updateLockedReward(ctx, reward)
		if err != nil {
			continue // Skip if storage fails
		}

		// Emit unlock event
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"retail_reward_unlocked",
				sdk.NewAttribute("recipient", reward.Recipient),
				sdk.NewAttribute("amount", reward.Amount.String()),
				sdk.NewAttribute("unlock_time", reward.UnlockTime.Format(time.RFC3339)),
			),
		)
	}

	return nil
}

// DistributePOSPaymentReward distributes POS payment fee: 50% treasury, 50% retail wallet (locked 6mo)
func (k Keeper) DistributePOSPaymentReward(ctx sdk.Context, totalFee sdk.Coin, retailWallet sdk.AccAddress) error {
	// Split fee 50:50
	halfAmount := totalFee.Amount.QuoRaw(2)
	treasuryAmount := sdk.NewCoin(totalFee.Denom, halfAmount)
	retailAmount := sdk.NewCoin(totalFee.Denom, halfAmount)

	// Send 50% to treasury
	treasuryAddr := k.GetModuleAddress("treasury")
	err := k.bankKeeper.SendCoins(ctx, k.GetModuleAddress(types.ModuleName), treasuryAddr, sdk.NewCoins(treasuryAmount))
	if err != nil {
		return sdkerrors.ErrInsufficientFunds.Wrapf("failed to send to treasury: %v", err)
	}

	// Lock 50% for retail wallet (6 months)
	err = k.LockRetailWalletReward(ctx, retailWallet, retailAmount)
	if err != nil {
		return err
	}

	// Emit distribution event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"pos_payment_fee_distributed",
			sdk.NewAttribute("total_fee", totalFee.String()),
			sdk.NewAttribute("treasury_amount", treasuryAmount.String()),
			sdk.NewAttribute("retail_locked_amount", retailAmount.String()),
			sdk.NewAttribute("retail_wallet", retailWallet.String()),
		),
	)

	return nil
}

// Helper function to get module address
func (k Keeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return k.bankKeeper.GetModuleAddress(moduleName)
}

// Storage helpers - these would need to be implemented with proper collections
func (k Keeper) storeLockedReward(ctx sdk.Context, reward LockedReward) error {
	// TODO: Implement proper storage with collections
	// For now, this is a placeholder
	return nil
}

func (k Keeper) updateLockedReward(ctx sdk.Context, reward LockedReward) error {
	// TODO: Implement proper storage with collections
	// For now, this is a placeholder
	return nil
}

func (k Keeper) getAllLockedRewards(ctx sdk.Context) ([]LockedReward, error) {
	// TODO: Implement proper storage with collections
	// For now, this is a placeholder
	return []LockedReward{}, nil
}

// GetLockedRewardsByRecipient returns all locked rewards for a specific recipient
func (k Keeper) GetLockedRewardsByRecipient(ctx sdk.Context, recipient sdk.AccAddress) ([]LockedReward, error) {
	// TODO: Implement proper storage with collections
	// For now, this is a placeholder
	return []LockedReward{}, nil
}

// GetTotalLockedRewards returns the total amount of locked rewards
func (k Keeper) GetTotalLockedRewards(ctx sdk.Context) (sdk.Coins, error) {
	lockedRewardsAddr := k.GetModuleAddress("locked_rewards")
	return k.bankKeeper.GetAllBalances(ctx, lockedRewardsAddr), nil
}
