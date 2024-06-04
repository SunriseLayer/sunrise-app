package swap

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	modulev1 "github.com/sunriselayer/sunrise/api/sunrise/swap"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: modulev1.Query_ServiceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Shows the parameters of the module",
				},
				{
					RpcMethod: "InFlightPacketAll",
					Use:       "list-in-flight-packet",
					Short:     "List all in-flight-packet",
				},
				{
					RpcMethod:      "InFlightPacket",
					Use:            "show-in-flight-packet [src-port] [src-channel] [sequence]",
					Short:          "Shows a in-flight-packet",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "src_port_id"}, {ProtoField: "src_channel_id"}, {ProtoField: "sequence"}},
				},
				{
					RpcMethod: "AckWaitingPacketAll",
					Use:       "list-ack-waiting-packet",
					Short:     "List all ack-waiting-packet",
				},
				{
					RpcMethod:      "AckWaitingPacket",
					Use:            "show-ack-waiting-packet [id]",
					Short:          "Shows a ack-waiting-packet",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "index"}},
				},
				// this line is used by ignite scaffolding # autocli/query
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              modulev1.Msg_ServiceDesc.ServiceName,
			EnhanceCustomCommand: true, // only required if you want to use the custom command
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // skipped because authority gated
				},
				{
					RpcMethod:      "SwapExactAmountIn",
					Use:            "swap-exact-amount-in",
					Short:          "Send a swap-exact-amount-in tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						// {ProtoField: "interface_provider"},
						// {ProtoField: "route"},
						// {ProtoField: "amount_in"},
						// {ProtoField: "min_amount_out"},
					},
				},
				{
					RpcMethod:      "SwapExactAmountOut",
					Use:            "swap-exact-amount-out",
					Short:          "Send a swap-exact-amount-out tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						// {ProtoField: "interface_provider"},
						// {ProtoField: "route"},
						// {ProtoField: "max_amount_in"},
						// {ProtoField: "amount_out"},
					},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
