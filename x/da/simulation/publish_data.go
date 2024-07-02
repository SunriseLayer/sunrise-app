package simulation

import (
	"math/rand"

	"github.com/sunriselayer/sunrise/x/da/keeper"
	"github.com/sunriselayer/sunrise/x/da/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

func SimulateMsgPublishData(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgPublishData{
			Sender: simAccount.Address.String(),
		}

		// TODO: Handling the PublishData simulation

		return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "PublishData simulation not implemented"), nil, nil
	}
}
