package cmd

import (
	"os"

	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtxconfig "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/spf13/cobra"

	"cosvian/app"
	osmosisicqtypes "cosvian/x/osmosisicq/types"
)

// NewRootCmd creates a new root command for cosviand. It is called once in the main function.
func NewRootCmd() *cobra.Command {
	var (
		autoCliOpts        autocli.AppOptions
		moduleBasicManager module.BasicManager
		clientCtx          client.Context
	)

	if err := depinject.Inject(
		depinject.Configs(app.AppConfig(),
			depinject.Supply(log.NewNopLogger()),
			depinject.Provide(
				ProvideClientContext,
			),
		),
		&autoCliOpts,
		&moduleBasicManager,
		&clientCtx,
	); err != nil {
		panic(err)
	}

	rootCmd := &cobra.Command{
		Use:           app.Name + "d",
		Short:         "cosvian node",
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			clientCtx = clientCtx.WithCmdContext(cmd.Context()).WithViper(app.Name)
			clientCtx, err := client.ReadPersistentCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			clientCtx, err = config.ReadFromClientConfig(clientCtx)
			if err != nil {
				return err
			}

			if err := client.SetCmdClientContextHandler(clientCtx, cmd); err != nil {
				return err
			}

			customAppTemplate, customAppConfig := initAppConfig()
			customCMTConfig := initCometBFTConfig()

			return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customCMTConfig)
		},
	}

	// Since the IBC modules don't support dependency injection, we need to
	// manually register the modules on the client side.
	// This needs to be removed after IBC supports App Wiring.
	ibcModules := app.RegisterIBC(clientCtx.Codec)
	for name, mod := range ibcModules {
		moduleBasicManager[name] = module.CoreAppModuleBasicAdaptor(name, mod)
		autoCliOpts.Modules[name] = mod
	}

	initRootCmd(rootCmd, clientCtx.TxConfig, moduleBasicManager)

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	// Fallback: if auto-cli did not inject the osmosisicq query namespace, add it manually.
	if add := ensureOsmosisICQQueryFallback(rootCmd, clientCtx); add {
		// no-op; function performs registration when needed
	}

	return rootCmd
}

// addOsmosisICQManualQueries injects a lightweight manual query command group for the
// osmosisicq module when auto-cli generation does not expose it. Safe to call even if
// auto-cli later gains support (guarded by existence check above).
func addOsmosisICQManualQueries(root *cobra.Command, clientCtx client.Context) {
	queryCmd, _, err := root.Find([]string{"query"})
	if err != nil || queryCmd == nil {
		return
	}

	osmoCmd := &cobra.Command{
		Use:   "osmosisicq",
		Short: "Querying commands for the osmosisicq module (manual fallback)",
	}

	paramsCmd := &cobra.Command{
		Use:   "params",
		Short: "Shows the parameters of the osmosisicq module",
		RunE: func(cmd *cobra.Command, args []string) error {
			qc := osmosisicqtypes.NewQueryClient(clientCtx)
			res, err := qc.Params(cmd.Context(), &osmosisicqtypes.QueryParamsRequest{})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Shows scheduler timing and registered ICQ query IDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			qc := osmosisicqtypes.NewQueryClient(clientCtx)
			res, err := qc.Status(cmd.Context(), &osmosisicqtypes.QueryStatusRequest{})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}

	osmoCmd.AddCommand(paramsCmd, statusCmd)
	queryCmd.AddCommand(osmoCmd)
}

// ensureOsmosisICQQueryFallback checks presence of the osmosisicq query group; if absent, registers it.
// Returns true if it added the group.
func ensureOsmosisICQQueryFallback(root *cobra.Command, clientCtx client.Context) bool {
	queryCmd, _, err := root.Find([]string{"query"})
	if err != nil || queryCmd == nil {
		return false
	}
	for _, c := range queryCmd.Commands() {
		if c.Name() == "osmosisicq" {
			return false // already present
		}
	}
	addOsmosisICQManualQueries(root, clientCtx)
	return true
}

// ProvideClientContext creates and provides a fully initialized client.Context,
// allowing it to be used for dependency injection and CLI operations.
func ProvideClientContext(
	appCodec codec.Codec,
	interfaceRegistry codectypes.InterfaceRegistry,
	txConfigOpts tx.ConfigOptions,
	legacyAmino *codec.LegacyAmino,
) client.Context {
	// Defensive registration: ensure custom osmosisicq Msg types are available to CLI JSON decoding
	// even if wiring misses them in certain build/tag scenarios.
	osmosisicqtypes.RegisterInterfaces(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(appCodec).
		WithInterfaceRegistry(interfaceRegistry).
		WithLegacyAmino(legacyAmino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper(app.Name) // env variable prefix

	// Read the config again to overwrite the default values with the values from the config file
	clientCtx, _ = config.ReadFromClientConfig(clientCtx)

	// textual is enabled by default, we need to re-create the tx config grpc instead of bank keeper.
	txConfigOpts.TextualCoinMetadataQueryFn = authtxconfig.NewGRPCCoinMetadataQueryFn(clientCtx)
	txConfig, err := tx.NewTxConfigWithOptions(clientCtx.Codec, txConfigOpts)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithTxConfig(txConfig)

	return clientCtx
}
