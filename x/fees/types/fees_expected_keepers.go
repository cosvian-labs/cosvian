package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FeesKeeper defines the expected interface that other modules can use to
// interact with the fees module.
type FeesKeeper interface {
	// ConvertUSDToBTO converts a USD-denominated LegacyDec into a CSV-denominated LegacyDec
	// and returns price data used for the conversion.
	ConvertUSDToBTO(ctx sdk.Context, usd math.LegacyDec) (math.LegacyDec, *PriceData, error)

	// GetFeeByType returns the configured USD fee amount for a fee category.
	GetFeeByType(ctx sdk.Context, feeType string) math.LegacyDec
	// DistributeFee handles distribution of a already-collected fee coin according
	// to the configured FeeTableUSD splits for the given feeType.
	DistributeFee(ctx sdk.Context, sender sdk.AccAddress, feeType string, feeCoin sdk.Coin, metadata map[string]interface{}) error
}
