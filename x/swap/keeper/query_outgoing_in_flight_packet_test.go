package keeper_test

import (
	"strconv"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	keepertest "github.com/sunriselayer/sunrise/testutil/keeper"
	"github.com/sunriselayer/sunrise/testutil/nullify"
	"github.com/sunriselayer/sunrise/x/swap/types"
)

// Prevent strconv unused error
var _ = strconv.IntSize

func TestOutgoingInFlightPacketQuerySingle(t *testing.T) {
	keeper, ctx := keepertest.SwapKeeper(t)
	msgs := createNOutgoingInFlightPacket(keeper, ctx, 2)
	tests := []struct {
		desc     string
		request  *types.QueryGetOutgoingInFlightPacketRequest
		response *types.QueryGetOutgoingInFlightPacketResponse
		err      error
	}{
		{
			desc: "First",
			request: &types.QueryGetOutgoingInFlightPacketRequest{
				SrcPortId:    msgs[0].Index.PortId,
				SrcChannelId: msgs[0].Index.ChannelId,
				Sequence:     msgs[0].Index.Sequence,
			},
			response: &types.QueryGetOutgoingInFlightPacketResponse{OutgoingInFlightPacket: msgs[0]},
		},
		{
			desc: "Second",
			request: &types.QueryGetOutgoingInFlightPacketRequest{
				SrcPortId:    msgs[1].Index.PortId,
				SrcChannelId: msgs[1].Index.ChannelId,
				Sequence:     msgs[1].Index.Sequence,
			},
			response: &types.QueryGetOutgoingInFlightPacketResponse{OutgoingInFlightPacket: msgs[1]},
		},
		{
			desc: "KeyNotFound",
			request: &types.QueryGetOutgoingInFlightPacketRequest{
				SrcPortId:    strconv.Itoa(100000),
				SrcChannelId: strconv.Itoa(100000),
				Sequence:     uint64(100000),
			},
			err: status.Error(codes.NotFound, "not found"),
		},
		{
			desc: "InvalidRequest",
			err:  status.Error(codes.InvalidArgument, "invalid request"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			response, err := keeper.OutgoingInFlightPacket(ctx, tc.request)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t,
					nullify.Fill(tc.response),
					nullify.Fill(response),
				)
			}
		})
	}
}

func TestOutgoingInFlightPacketQueryPaginated(t *testing.T) {
	keeper, ctx := keepertest.SwapKeeper(t)
	msgs := createNOutgoingInFlightPacket(keeper, ctx, 5)

	request := func(next []byte, offset, limit uint64, total bool) *types.QueryAllOutgoingInFlightPacketRequest {
		return &types.QueryAllOutgoingInFlightPacketRequest{
			Pagination: &query.PageRequest{
				Key:        next,
				Offset:     offset,
				Limit:      limit,
				CountTotal: total,
			},
		}
	}
	t.Run("ByOffset", func(t *testing.T) {
		step := 2
		for i := 0; i < len(msgs); i += step {
			resp, err := keeper.OutgoingInFlightPacketAll(ctx, request(nil, uint64(i), uint64(step), false))
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp.OutgoingInFlightPacket), step)
			require.Subset(t,
				nullify.Fill(msgs),
				nullify.Fill(resp.OutgoingInFlightPacket),
			)
		}
	})
	t.Run("ByKey", func(t *testing.T) {
		step := 2
		var next []byte
		for i := 0; i < len(msgs); i += step {
			resp, err := keeper.OutgoingInFlightPacketAll(ctx, request(next, 0, uint64(step), false))
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp.OutgoingInFlightPacket), step)
			require.Subset(t,
				nullify.Fill(msgs),
				nullify.Fill(resp.OutgoingInFlightPacket),
			)
			next = resp.Pagination.NextKey
		}
	})
	t.Run("Total", func(t *testing.T) {
		resp, err := keeper.OutgoingInFlightPacketAll(ctx, request(nil, 0, 0, true))
		require.NoError(t, err)
		require.Equal(t, len(msgs), int(resp.Pagination.Total))
		require.ElementsMatch(t,
			nullify.Fill(msgs),
			nullify.Fill(resp.OutgoingInFlightPacket),
		)
	})
	t.Run("InvalidRequest", func(t *testing.T) {
		_, err := keeper.OutgoingInFlightPacketAll(ctx, nil)
		require.ErrorIs(t, err, status.Error(codes.InvalidArgument, "invalid request"))
	})
}
