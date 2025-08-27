# Bitora Fee Table Mechanism

This module implements a comprehensive fee table mechanism for the Bitora blockchain with USD-anchored fees converted to BTO using Band Protocol oracle prices.

## Features

### 1. Fee Categories
- **POS Payment**: $0.15 USD
- **Token Interaction**: $3.00 USD  
- **Native Transfer**: $1.00 USD
- **DEX Native**: $1.00 USD
- **DEX User**: $3.00 USD
- **Deploy**: Free (with gas cap)
- **Wizard**: Free (with gas cap)

### 2. Oracle Integration
- Band Protocol integration for BTO/USD price feeds
- TWAP (Time-Weighted Average Price) calculation
- Fallback price mechanism when oracle is unavailable
- Price freshness validation and deviation limits

### 3. Fee Calculation
- USD fees converted to BTO using oracle prices
- Conservative rounding (ceil) to prevent under-collection
- Gas price calculated as: `gasPrice = feeBTO / gasWanted`
- Guard rails for min/max gas prices

### 4. Free Tier Support
- Deploy and Wizard transactions are free
- Gas caps enforced for free tier transactions
- Abuse prevention through strict limits

### 5. Fee Distribution
- Configurable splits per category:
  - Treasury
  - Retail Wallet
  - Token Developer  
  - Token Creator

## Implementation Status

### ✅ Completed Components

1. **Module Structure**: Created using Ignite scaffold
2. **Protobuf Definitions**: Complete fee table, oracle params, guard rails
3. **Default Parameters**: USD fee amounts and distribution splits
4. **Oracle Adapter**: Band Protocol integration with TWAP and fallback
5. **Fee Calculator**: Transaction classification and fee estimation
6. **AnteHandler**: Fee validation and deduction (partial)

### 🚧 In Progress

1. **AnteHandler**: Fixing lint errors and compilation issues
2. **Error Handling**: Updating to newer Cosmos SDK error patterns

### 📋 Pending

1. **Fee Distribution**: Complete implementation of splits
2. **Integration Testing**: End-to-end testing with blockchain
3. **Documentation**: API documentation and usage examples

## Key Files

- `x/fees/types/params.proto` - Fee table and parameter definitions
- `x/fees/keeper/oracle_adapter.go` - Band Protocol integration
- `x/fees/keeper/fee_calculator.go` - Fee calculation logic
- `x/fees/keeper/ante_handler.go` - Fee validation and deduction
- `x/fees/types/params.go` - Default parameters and validation

## Usage

### For SDK/Wallet Developers

```go
// Estimate fee for a transaction
feeCalculator := keeper.NewFeeCalculator(keeper, oracleAdapter)
estimate, err := feeCalculator.EstimateFee(ctx, msgs, memo, gasWanted)

// Build transaction with calculated fee
fee := feeCalculator.BuildFeeFromEstimate(estimate, "ubto")
```

### Transaction Categories

Transactions are automatically classified based on:
1. **Message types** (bank.MsgSend, wasm.MsgExecuteContract, etc.)
2. **Memo content** (keywords like "pos", "dex", "deploy")
3. **Contract interaction patterns** (token operations, DEX swaps)

### Gas and Fee Display

- `gas_used` and `gas_wanted` fields show **BTO equivalent amounts**
- Actual gas consumption is abstracted from users
- Fees are predictable and flat per category

## Configuration

### Oracle Parameters
```go
OracleParams{
    BandRequestId: 1,
    TwapWindow: 1 * time.Hour,
    DeviationLimit: sdk.NewDecWithPrec(10, 2), // 10%
    FallbackTtl: 24 * time.Hour,
    MaxPriceAge: 5 * time.Minute,
}
```

### Guard Rails
```go
GuardRails{
    MinGasPriceBto: sdk.NewDecWithPrec(1, 6),    // 0.000001 BTO
    MaxGasPriceBto: sdk.NewDecWithPrec(1000, 6), // 0.001 BTO  
    MaxGasWizard: 2000000,   // 2M gas for wizard
    MaxGasDeploy: 10000000,  // 10M gas for deploy
}
```

## Events

### Oracle Events
- `oracle_used`: Emitted when oracle price is fetched
- Includes price, TWAP, freshness, and fallback status

### Fee Events  
- `fee_charged`: Emitted when fee is deducted
- Includes category, USD amount, BTO amount, and metadata

## Error Handling

- Graceful fallback when oracle is unavailable
- Validation of fee amounts against estimates
- Gas cap enforcement for free tier transactions
- Guard rail validation for gas prices

## Security Considerations

- Conservative rounding prevents under-collection
- Price deviation limits prevent oracle manipulation
- Free tier gas caps prevent abuse
- Fee validation in AnteHandler ensures proper payment
