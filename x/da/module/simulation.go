package da

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/sunriselayer/sunrise/testutil/sample"
	dasimulation "github.com/sunriselayer/sunrise/x/da/simulation"
	"github.com/sunriselayer/sunrise/x/da/types"
)

// avoid unused import issue
var (
	_ = dasimulation.FindAccount
	_ = rand.Rand{}
	_ = sample.AccAddress
	_ = sdk.AccAddress{}
	_ = simulation.MsgEntryKind
)

const (
    opWeightMsgPublishData = "op_weight_msg_publish_data"
	// TODO: Determine the simulation weight value
	defaultWeightMsgPublishData int = 100

	// this line is used by starport scaffolding # simapp/module/const
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	daGenesis := types.GenesisState{
		Params:	types.DefaultParams(),
		// this line is used by starport scaffolding # simapp/module/genesisState
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&daGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgPublishData int
	simState.AppParams.GetOrGenerate(opWeightMsgPublishData, &weightMsgPublishData, nil,
		func(_ *rand.Rand) {
			weightMsgPublishData = defaultWeightMsgPublishData
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgPublishData,
		dasimulation.SimulateMsgPublishData(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	// this line is used by starport scaffolding # simapp/module/operation

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{
	    simulation.NewWeightedProposalMsg(
	opWeightMsgPublishData,
	defaultWeightMsgPublishData,
	func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
		dasimulation.SimulateMsgPublishData(am.accountKeeper, am.bankKeeper, am.keeper)
		return nil
	},
),
// this line is used by starport scaffolding # simapp/module/OpMsg
	}
}
