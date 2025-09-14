//go:build test

package keeper

import (
    sdk "github.com/cosmos/cosmos-sdk/types"
)

// testAckExtractor allows tests to inject a custom ack decoder without importing async-icq.
var testAckExtractor func(ctx sdk.Context, acknowledgement []byte) ([]byte, error)

// SetTestAckExtractor sets the test ack extractor; tests should reset to nil when done.
func SetTestAckExtractor(f func(ctx sdk.Context, acknowledgement []byte) ([]byte, error)) {
    testAckExtractor = f
}
