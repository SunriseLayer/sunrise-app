package keeper

import (
	"context"

    "github.com/sunriselayer/sunrise/x/da/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)


func (k msgServer) PublishData(goCtx context.Context,  msg *types.MsgPublishData) (*types.MsgPublishDataResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

    // TODO: Handling the message
    _ = ctx

	return &types.MsgPublishDataResponse{}, nil
}
