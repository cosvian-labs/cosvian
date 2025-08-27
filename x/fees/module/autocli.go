package fees

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"bitora/x/fees/types"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Shows the parameters of the module",
				},
				{
					RpcMethod:      "OraclePrice",
					Use:            "oracle-price ",
					Short:          "Query oracle-price",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{},
				},

				// this line is used by ignite scaffolding # autocli/query
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true, // only required if you want to use the custom command
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // skipped because authority gated
				},
				{
					RpcMethod:      "ChargeFee",
					Use:            "charge-fee [category] [amount] [metadata]",
					Short:          "Send a charge-fee tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "category"}, {ProtoField: "amount"}, {ProtoField: "metadata"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
