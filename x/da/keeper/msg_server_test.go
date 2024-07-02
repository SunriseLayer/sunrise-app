package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

    keepertest "github.com/sunriselayer/sunrise/testutil/keeper"
    "github.com/sunriselayer/sunrise/x/da/types"
    "github.com/sunriselayer/sunrise/x/da/keeper"
)

func setupMsgServer(t testing.TB) (keeper.Keeper, types.MsgServer, context.Context) {
	k, ctx := keepertest.DaKeeper(t)
	return k, keeper.NewMsgServerImpl(k), ctx
}

func TestMsgServer(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)
	require.NotNil(t, ms)
	require.NotNil(t, ctx)
	require.NotEmpty(t, k)
}