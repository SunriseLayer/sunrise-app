package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/sunrise-zone/sunrise-app/app"
	"github.com/sunrise-zone/sunrise-app/app/encoding"
	"github.com/sunrise-zone/sunrise-app/pkg/user"
	util "github.com/sunrise-zone/sunrise-app/test/util"
	"github.com/sunrise-zone/sunrise-app/test/util/blobfactory"
	"github.com/sunrise-zone/sunrise-app/test/util/testnode"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/rand"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestSignerTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode.")
	}
	suite.Run(t, new(SignerTestSuite))
}

type SignerTestSuite struct {
	suite.Suite

	ctx    testnode.Context
	encCfg encoding.Config
	signer *user.Signer
}

func (s *SignerTestSuite) SetupSuite() {
	s.encCfg = encoding.MakeConfig(util.ModuleBasics)
	s.ctx, _, _ = testnode.NewNetwork(s.T(), testnode.DefaultConfig().WithFundedAccounts("a"))
	_, err := s.ctx.WaitForHeight(1)
	s.Require().NoError(err)
	rec, err := s.ctx.Keyring.Key("a")
	s.Require().NoError(err)
	addr, err := rec.GetAddress()
	s.Require().NoError(err)
	s.signer, err = user.SetupSigner(s.ctx.GoContext(), s.ctx.Keyring, s.ctx.GRPCClient, addr, s.encCfg)
	s.Require().NoError(err)
}

func (s *SignerTestSuite) TestSubmitPayForBlob() {
	t := s.T()
	blobs := blobfactory.ManyRandBlobs(rand.NewRand(), 1e3, 1e4)
	fee := user.SetFee(1e6)
	gas := user.SetGasLimit(1e6)
	subCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := s.signer.SubmitPayForBlob(subCtx, blobs, fee, gas)
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Code)
}

func (s *SignerTestSuite) TestSubmitTx() {
	t := s.T()
	fee := user.SetFee(1e6)
	gas := user.SetGasLimit(1e6)
	msg := bank.NewMsgSend(s.signer.Address(), testnode.RandomAddress().(sdk.AccAddress), sdk.NewCoins(sdk.NewInt64Coin(app.BondDenom, 10)))
	resp, err := s.signer.SubmitTx(s.ctx.GoContext(), []sdk.Msg{msg}, fee, gas)
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Code)
}

func (s *SignerTestSuite) ConfirmTxTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := s.signer.ConfirmTx(ctx, string("E32BD15CAF57AF15D17B0D63CF4E63A9835DD1CEBB059C335C79586BC3013728"))
	require.Error(s.T(), err)
	require.Equal(s.T(), err, context.DeadlineExceeded)
}

// TestGasConsumption verifies that the amount deducted from a user's balance is
// based on the fee provided in the tx instead of the gas used by the tx. This
// behavior leads to poor UX because tx submitters must over-estimate the amount
// of gas that their tx will consume and they are not refunded for the excess.
func (s *SignerTestSuite) TestGasConsumption() {
	t := s.T()

	usrToSend := int64(1)
	msg := bank.NewMsgSend(s.signer.Address(), testnode.RandomAddress().(sdk.AccAddress), sdk.NewCoins(sdk.NewInt64Coin(app.BondDenom, usrToSend)))

	gasPrice := int64(1)
	gasLimit := uint64(1e6)
	fee := uint64(1e6) // 1 TIA
	// Note: gas price * gas limit = fee amount. So by setting gasLimit and fee
	// to the same value, these options set a gas price of 1usr.
	options := []user.TxOption{user.SetGasLimit(gasLimit), user.SetFee(fee)}

	balanceBefore := s.queryCurrentBalance(t)
	resp, err := s.signer.SubmitTx(s.ctx.GoContext(), []sdk.Msg{msg}, options...)
	require.NoError(t, err)

	require.EqualValues(t, abci.CodeTypeOK, resp.Code)
	balanceAfter := s.queryCurrentBalance(t)

	// verify that the amount deducted depends on the fee set in the tx.
	amountDeducted := balanceBefore - balanceAfter - usrToSend
	assert.Equal(t, int64(fee), amountDeducted)

	// verify that the amount deducted does not depend on the actual gas used.
	gasUsedBasedDeduction := resp.GasUsed * gasPrice
	assert.NotEqual(t, gasUsedBasedDeduction, amountDeducted)
	// The gas used based deduction should be less than the fee because the fee is 1 TIA.
	assert.Less(t, gasUsedBasedDeduction, int64(fee))
}

func (s *SignerTestSuite) queryCurrentBalance(t *testing.T) int64 {
	balanceQuery := bank.NewQueryClient(s.ctx.GRPCClient)
	balanceResp, err := balanceQuery.AllBalances(s.ctx.GoContext(), &bank.QueryAllBalancesRequest{Address: s.signer.Address().String()})
	require.NoError(t, err)
	return balanceResp.Balances.AmountOf(app.BondDenom).Int64()
}
