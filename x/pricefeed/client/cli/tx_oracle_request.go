package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channelutils "github.com/cosmos/ibc-go/v10/modules/core/04-channel/client/utils"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"cosvian/x/pricefeed/types"
)

// CmdSendOracleRequest() returns the OracleRequest send packet command.
// This command does not use AutoCLI because it gives a better UX to do not.
func CmdSendOracleRequest() *cobra.Command {
	flagPacketTimeoutTimestamp := "packet-timeout-timestamp"

	cmd := &cobra.Command{
		Use:   "send-oracle-request [src-port] [src-channel] [oracle-script-id] [calldata] [symbols] [ask-count] [min-count] [fee-limit] [prepare-gas] [execute-gas] [client-id]",
		Short: "Send a oracle-request over IBC",
		Args:  cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			creator := clientCtx.GetFromAddress().String()
			srcPort := args[0]
			srcChannel := args[1]

			argOracleScriptId, err := cast.ToUint64E(args[2])
			if err != nil {
				return err
			}
			argCalldata := args[3]
			argSymbols := args[4]
			argAskCount, err := cast.ToUint64E(args[5])
			if err != nil {
				return err
			}
			argMinCount, err := cast.ToUint64E(args[6])
			if err != nil {
				return err
			}
			argFeeLimit := args[7]
			argPrepareGas, err := cast.ToUint64E(args[8])
			if err != nil {
				return err
			}
			argExecuteGas, err := cast.ToUint64E(args[9])
			if err != nil {
				return err
			}
			argClientId := args[10]

			// Get the relative timeout timestamp
			timeoutTimestamp, err := cmd.Flags().GetUint64(flagPacketTimeoutTimestamp)
			if err != nil {
				return err
			}

			clientRes, err := channelutils.QueryChannelClientState(clientCtx, srcPort, srcChannel, false)
			if err != nil {
				return err
			}

			var clientState exported.ClientState
			if err := clientCtx.InterfaceRegistry.UnpackAny(clientRes.IdentifiedClientState.ClientState, &clientState); err != nil {
				return err
			}

			consensusStateAny, err := channelutils.QueryChannelConsensusState(clientCtx, srcPort, srcChannel, clienttypes.Height{}, false)
			if err != nil {
				return err
			}

			var consensusState exported.ConsensusState
			if err := clientCtx.InterfaceRegistry.UnpackAny(consensusStateAny.GetConsensusState(), &consensusState); err != nil {
				return err
			}

			if timeoutTimestamp != 0 {
				timeoutTimestamp = consensusState.GetTimestamp() + timeoutTimestamp //nolint:staticcheck // client side
			}

			msg := types.NewMsgSendOracleRequest(creator, srcPort, srcChannel, timeoutTimestamp, argOracleScriptId, argCalldata, argSymbols, argAskCount, argMinCount, argFeeLimit, argPrepareGas, argExecuteGas, argClientId)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64(flagPacketTimeoutTimestamp, DefaultRelativePacketTimeoutTimestamp, "Packet timeout timestamp in nanoseconds. Default is 10 minutes.")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
