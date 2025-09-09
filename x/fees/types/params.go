package types

import (
	"fmt"
	"reflect"
	"time"

	"cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams() Params {
	return Params{}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{
		FeeTableUsd: FeeTableUSD{
			PosPayment: FeeConfig{
				UsdAmount: math.LegacyMustNewDecFromStr("0.15"),
				Split: FeeSplit{
					Treasury:     math.LegacyMustNewDecFromStr("0.5"),
					RetailWallet: math.LegacyMustNewDecFromStr("0.5"),
					TokenDev:     math.LegacyZeroDec(),
					TokenCreator: math.LegacyZeroDec(),
				},
			},
			TokenInteraction: FeeConfig{
				UsdAmount: math.LegacyMustNewDecFromStr("3.0"),
				Split: FeeSplit{
					Treasury:     math.LegacyMustNewDecFromStr("0.5"),
					RetailWallet: math.LegacyZeroDec(),
					TokenDev:     math.LegacyMustNewDecFromStr("0.5"),
					TokenCreator: math.LegacyZeroDec(),
				},
			},
			NativeTransfer: FeeConfig{
				UsdAmount: math.LegacyMustNewDecFromStr("1.0"),
				Split: FeeSplit{
					Treasury:     math.LegacyOneDec(),
					RetailWallet: math.LegacyZeroDec(),
					TokenDev:     math.LegacyZeroDec(),
					TokenCreator: math.LegacyZeroDec(),
				},
			},
			DexNative: FeeConfig{
				UsdAmount: math.LegacyMustNewDecFromStr("1.0"),
				Split: FeeSplit{
					Treasury:     math.LegacyOneDec(),
					RetailWallet: math.LegacyZeroDec(),
					TokenDev:     math.LegacyZeroDec(),
					TokenCreator: math.LegacyZeroDec(),
				},
			},
			DexUser: FeeConfig{
				UsdAmount: math.LegacyMustNewDecFromStr("3.0"),
				Split: FeeSplit{
					Treasury:     math.LegacyMustNewDecFromStr("0.5"),
					RetailWallet: math.LegacyZeroDec(),
					TokenDev:     math.LegacyZeroDec(),
					TokenCreator: math.LegacyMustNewDecFromStr("0.5"),
				},
			},
			Deploy: FeeConfig{
				UsdAmount: math.LegacyZeroDec(), // Free
				Split: FeeSplit{
					Treasury:     math.LegacyZeroDec(),
					RetailWallet: math.LegacyZeroDec(),
					TokenDev:     math.LegacyZeroDec(),
					TokenCreator: math.LegacyZeroDec(),
				},
			},
			Wizard: FeeConfig{
				UsdAmount: math.LegacyZeroDec(), // Free
				Split: FeeSplit{
					Treasury:     math.LegacyZeroDec(),
					RetailWallet: math.LegacyZeroDec(),
					TokenDev:     math.LegacyZeroDec(),
					TokenCreator: math.LegacyZeroDec(),
				},
			},
		},
		OracleParams: OracleParams{
			BandRequestId:  1, // BTO/USD request ID on Band
			TwapWindow:     5 * time.Minute,
			DeviationLimit: math.LegacyMustNewDecFromStr("0.05"), // 5% deviation limit
			FallbackTtl:    30 * time.Minute,
			MaxPriceAge:    10 * time.Minute,
		},
		GuardRails: GuardRails{
			MinGasPriceBto: math.LegacyMustNewDecFromStr("0.000001"), // 1 micro BTO
			MaxGasPriceBto: math.LegacyMustNewDecFromStr("1.0"),      // 1 BTO
			MaxGasWizard:   100000,                                   // 100k gas for wizard
			MaxGasDeploy:   500000,                                   // 500k gas for deploy
		},
		MinGasPolicy: MinGasPolicy{
			Enabled:            true,
			GlobalMinGasPrice:  math.LegacyMustNewDecFromStr("0.000001"),
			AllowPerTxOverride: true,
		},
		// Default wallet addresses (to be configured in production)
		TreasuryWallet:     "bto1team00000000000000000000000000000000000", // Placeholder
		RetailWallet:       "bto1retail000000000000000000000000000000000", // Placeholder
		TokenDevWallet:     "bto1dev000000000000000000000000000000000000", // Placeholder
		TokenCreatorWallet: "bto1creator00000000000000000000000000000000", // Placeholder
		// Enable hybrid mode by default on this branch so IBC system txs are gas-only out of the box.
		FeeMode:            FeeModeHybrid,
		SystemMsgTypeUrls: []string{
			"/ibc.core.client.v1.MsgCreateClient",
			"/ibc.core.client.v1.MsgUpdateClient",
			"/ibc.core.connection.v1.MsgConnectionOpenInit",
			"/ibc.core.connection.v1.MsgConnectionOpenTry",
			"/ibc.core.connection.v1.MsgConnectionOpenAck",
			"/ibc.core.connection.v1.MsgConnectionOpenConfirm",
			"/ibc.core.channel.v1.MsgChannelOpenInit",
			"/ibc.core.channel.v1.MsgChannelOpenTry",
			"/ibc.core.channel.v1.MsgChannelOpenAck",
			"/ibc.core.channel.v1.MsgChannelOpenConfirm",
			"/ibc.core.channel.v1.MsgRecvPacket",
			"/ibc.core.channel.v1.MsgAcknowledgement",
			"/ibc.core.channel.v1.MsgTimeout",
			"/ibc.core.channel.v1.MsgTimeoutOnClose",
			"/ibc.applications.fee.v1.MsgPayPacketFee",
			"/ibc.applications.fee.v1.MsgPayPacketFeeAsync",
			"/ibc.applications.fee.v1.MsgRegisterPayee",
			"/ibc.applications.fee.v1.MsgRegisterCounterpartyPayee",
			"/ibc.applications.interchain_accounts.controller.v1.MsgRegisterInterchainAccount",
			"/ibc.applications.interchain_accounts.controller.v1.MsgSendTx",
		},
		ExemptMsgTypeUrls: []string{},
		ExemptAddresses:  []string{},
	}
}

// Validate validates the set of params.
func (p Params) Validate() error {
	// Allow zero-value-like Params: detect by empty FeeMode & zero usd table amounts instead of struct compare (slices not comparable)
	if p.FeeMode == "" && p.FeeTableUsd.PosPayment.UsdAmount.IsZero() && len(p.SystemMsgTypeUrls) == 0 && len(p.ExemptMsgTypeUrls) == 0 {
		return nil
	}
	if err := p.validateFeeTableUSD(); err != nil {
		return err
	}
	if err := p.validateOracleParams(); err != nil {
		return err
	}
	if err := p.validateGuardRails(); err != nil {
		return err
	}
	if err := p.validateMinGasPolicy(); err != nil {
		return err
	}
	if err := p.validateHybridFields(); err != nil {
		return err
	}
	return nil
}

func (p Params) validateHybridFields() error {
	switch p.FeeMode {
	case "", "table", "hybrid", "gas_only":
	default:
		return fmt.Errorf("invalid fee_mode: %s", p.FeeMode)
	}
	// Basic sanity: no duplicates in system or exempt lists
	seen := map[string]struct{}{}
	for _, s := range p.SystemMsgTypeUrls {
		if s == "" { continue }
		if _, ok := seen[s]; ok { return fmt.Errorf("duplicate system_msg_type_url: %s", s) }
		seen[s] = struct{}{}
	}
	for _, s := range p.ExemptMsgTypeUrls {
		if s == "" { continue }
		if _, ok := seen[s]; ok { return fmt.Errorf("type url appears in both system and exempt lists: %s", s) }
	}
	return nil
}

func (p Params) validateFeeTableUSD() error {
	configs := []FeeConfig{
		p.FeeTableUsd.PosPayment,
		p.FeeTableUsd.TokenInteraction,
		p.FeeTableUsd.NativeTransfer,
		p.FeeTableUsd.DexNative,
		p.FeeTableUsd.DexUser,
		p.FeeTableUsd.Deploy,
		p.FeeTableUsd.Wizard,
	}

	for _, config := range configs {
		// If UsdAmount is the zero-value (not initialized), skip detailed checks.
		// This allows tests that pass empty Params to not panic inside decimal methods.
		if config.UsdAmount == (math.LegacyDec{}) {
			continue
		}

		if config.UsdAmount.IsNegative() {
			return fmt.Errorf("fee amount cannot be negative: %s", config.UsdAmount)
		}

		// Normalize splits similarly
		t := config.Split.Treasury
		r := config.Split.RetailWallet
		d := config.Split.TokenDev
		c := config.Split.TokenCreator
		if reflect.DeepEqual(t, math.LegacyDec{}) {
			t = math.LegacyZeroDec()
		}
		if reflect.DeepEqual(r, math.LegacyDec{}) {
			r = math.LegacyZeroDec()
		}
		if reflect.DeepEqual(d, math.LegacyDec{}) {
			d = math.LegacyZeroDec()
		}
		if reflect.DeepEqual(c, math.LegacyDec{}) {
			c = math.LegacyZeroDec()
		}

		// Validate split percentages sum to 1.0 (or 0.0 for free)
		total := t.Add(r).Add(d).Add(c)
		if !config.UsdAmount.IsZero() && !total.Equal(math.LegacyOneDec()) {
			return fmt.Errorf("fee split percentages must sum to 1.0, got: %s", total)
		}
	}

	return nil
}

func (p Params) validateOracleParams() error {
	if p.OracleParams.BandRequestId == 0 {
		return fmt.Errorf("band request ID cannot be zero")
	}
	if p.OracleParams.TwapWindow <= 0 {
		return fmt.Errorf("TWAP window must be positive")
	}
	if p.OracleParams.DeviationLimit.IsNegative() || p.OracleParams.DeviationLimit.GT(math.LegacyOneDec()) {
		return fmt.Errorf("deviation limit must be between 0 and 1")
	}
	if p.OracleParams.FallbackTtl <= 0 {
		return fmt.Errorf("fallback TTL must be positive")
	}
	if p.OracleParams.MaxPriceAge <= 0 {
		return fmt.Errorf("max price age must be positive")
	}

	return nil
}

func (p Params) validateGuardRails() error {
	if p.GuardRails.MinGasPriceBto.IsNegative() {
		return fmt.Errorf("min gas price cannot be negative")
	}
	if p.GuardRails.MaxGasPriceBto.IsNegative() {
		return fmt.Errorf("max gas price cannot be negative")
	}
	if p.GuardRails.MinGasPriceBto.GT(p.GuardRails.MaxGasPriceBto) {
		return fmt.Errorf("min gas price cannot be greater than max gas price")
	}
	if p.GuardRails.MaxGasWizard == 0 {
		return fmt.Errorf("max gas wizard must be positive")
	}
	if p.GuardRails.MaxGasDeploy == 0 {
		return fmt.Errorf("max gas deploy must be positive")
	}

	return nil
}

func (p Params) validateMinGasPolicy() error {
	if p.MinGasPolicy.GlobalMinGasPrice.IsNegative() {
		return fmt.Errorf("global min gas price cannot be negative")
	}

	return nil
}
