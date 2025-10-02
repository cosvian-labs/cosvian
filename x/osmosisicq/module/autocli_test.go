//go:build osmosis_removed

package osmosisicq

import (
	"testing"
)

// TestAutoCLIHasStatus ensures the Status RPC is exposed via autocli options.
func TestAutoCLIHasStatus(t *testing.T) {
	am := AppModule{}
	opts := am.AutoCLIOptions()
	if opts == nil || opts.Query == nil || len(opts.Query.RpcCommandOptions) == 0 {
		t.Fatalf("missing query options")
	}
	found := false
	for _, o := range opts.Query.RpcCommandOptions {
		if o != nil && o.RpcMethod == "Status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Status RPC not exposed in autocli options")
	}
}
