# Fee Table Integration Guide

## Current State Analysis

The Bitora blockchain currently uses a **zero-gas ante handler** that bypasses traditional gas consumption. Our fee table mechanism needs to integrate with this existing system while maintaining the predictable USD-based fee structure.

## Integration Strategy

### 1. Replace Zero-Gas with Fee Table Logic

The current `NewTransactionTypeFeeDecorator` and `NewZeroGasFeeDecorator` should be replaced with our fee table implementation:

```go
// Current (in app/ante.go):
NewTransactionTypeFeeDecorator(options.TokenKeeper),
NewZeroGasFeeDecorator(),

// Replace with:
NewFeeTableDecorator(options.FeesKeeper, options.OracleKeeper),
```

### 2. Update AnteHandlerOptions

Add the fees keeper to the ante handler options:

```go
type AnteHandlerOptions struct {
    ante.HandlerOptions
    
    AccountKeeper         authkeeper.AccountKeeper
    BankKeeper            types.BankKeeper
    TokenKeeper           tokenkeeper.Keeper
    FeesKeeper            feeskeeper.Keeper  // Add this
    IBCKeeper             *ibckeeper.Keeper
    // ... other fields
}
```

### 3. Fee Table Decorator Implementation

```go
// NewFeeTableDecorator creates a decorator that handles fee table logic
func NewFeeTableDecorator(feesKeeper feeskeeper.Keeper, oracleKeeper oracletypes.OracleKeeper) sdk.AnteDecorator {
    oracleAdapter := feeskeeper.NewOracleAdapter(feesKeeper, oracleKeeper)
    feeCalculator := feeskeeper.NewFeeCalculator(feesKeeper, oracleAdapter)
    
    return feeskeeper.NewFeeAnteHandler(feesKeeper, feeCalculator, bankKeeper)
}
```

## Key Changes Required

### 1. app/ante.go Updates

- Import fees keeper
- Add FeesKeeper to AnteHandlerOptions
- Replace zero-gas decorators with fee table decorator
- Maintain infinite gas meter for gas abstraction

### 2. app/app.go Updates

- Initialize fees keeper
- Wire dependencies (oracle keeper, bank keeper)
- Pass fees keeper to ante handler options

### 3. Gas Abstraction Strategy

Even with fee table, maintain gas abstraction:
- Use infinite gas meter in context
- Calculate fees based on transaction category, not gas consumption
- Display BTO equivalent amounts in gas fields for user clarity

## Migration Path

### Phase 1: Basic Integration
1. Add fees keeper to app.go
2. Update ante handler to use fee table
3. Test with existing transaction types

### Phase 2: Enhanced Features
1. Implement fee distribution
2. Add oracle price monitoring
3. Enable free tier for deploy/wizard

### Phase 3: UI/UX Updates
1. Update transaction receipts to show BTO equivalent
2. Modify wallet interfaces for fee display
3. Add fee estimation APIs

## Compatibility Considerations

### Existing Zero-Gas Behavior
- Maintain for backward compatibility
- Gradually migrate to fee table
- Provide configuration flags for transition

### Transaction Types
- Map existing transaction patterns to fee categories
- Ensure POS payments use $0.15 fee
- Token interactions use $3.00 fee
- Native transfers use $1.00 fee

### Error Handling
- Graceful fallback when oracle unavailable
- Clear error messages for insufficient fees
- Maintain transaction success rates during transition

## Testing Strategy

### Unit Tests
- Fee calculation accuracy
- Oracle price conversion
- Category classification
- Guard rail validation

### Integration Tests
- End-to-end transaction flow
- Fee deduction and distribution
- Oracle failure scenarios
- Free tier enforcement

### Performance Tests
- Oracle query latency
- Fee calculation overhead
- Memory usage optimization
- Concurrent transaction handling

## Deployment Checklist

- [ ] Update app.go with fees keeper
- [ ] Modify ante.go for fee table integration
- [ ] Configure oracle parameters
- [ ] Set up fee distribution accounts
- [ ] Test oracle connectivity
- [ ] Validate fee calculations
- [ ] Monitor transaction success rates
- [ ] Update documentation and APIs
