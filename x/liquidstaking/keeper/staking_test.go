package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/sunrise-zone/sunrise-app/testutil"
	"github.com/sunrise-zone/sunrise-app/x/liquidstaking/types"
)

var (
	// d is an alias for sdk.MustNewDecFromStr
	d = sdkmath.LegacyMustNewDecFromStr
	// i is an alias for sdkmath.NewInt
	i = sdkmath.NewInt
	// c is an alias for sdk.NewInt64Coin
	c = sdk.NewInt64Coin
)

func (suite *KeeperTestSuite) TestTransferDelegation_ValidatorStates() {
	_, addrs := testutil.GeneratePrivKeyAddressPairs(3)
	valAccAddr, fromDelegator, toDelegator := addrs[0], addrs[1], addrs[2]
	valAddr := sdk.ValAddress(valAccAddr)

	initialBalance := i(1e9)

	notBondedModAddr := authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName)
	bondedModAddr := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)

	testCases := []struct {
		name            string
		createValidator func() (delegatorShares sdkmath.LegacyDec, err error)
	}{
		{
			name: "bonded validator",
			createValidator: func() (sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, initialBalance)
				delegatorShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))

				// Run end blocker to update validator state to bonded.
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return delegatorShares, nil
			},
		},
		{
			name: "unbonded validator",
			createValidator: func() (sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, initialBalance)
				delegatorShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))

				// Don't run end blocker, new validators are by default unbonded.
				return delegatorShares, nil
			},
		},
		{
			name: "ubonding (jailed) validator",
			createValidator: func() (sdkmath.LegacyDec, error) {
				val := suite.CreateNewUnbondedValidator(valAddr, initialBalance)
				delegatorShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))

				// Run end blocker to update validator state to bonded.
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				// Jail and run end blocker to transition validator to unbonding.
				consAddr, err := val.GetConsAddr()
				if err != nil {
					return sdkmath.LegacyDec{}, err
				}
				suite.StakingKeeper.Jail(suite.Ctx, consAddr)
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return delegatorShares, nil
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			suite.CreateAccountWithAddress(valAccAddr, suite.NewBondCoins(i(1e9)))
			suite.CreateAccountWithAddress(fromDelegator, suite.NewBondCoins(i(1e9)))

			fromDelegationShares, err := tc.createValidator()
			suite.Require().NoError(err)

			validator, err := suite.StakingKeeper.GetValidator(suite.Ctx, valAddr)
			suite.Require().NoError(err)
			notBondedBalance := suite.BankKeeper.GetAllBalances(suite.Ctx, notBondedModAddr)
			bondedBalance := suite.BankKeeper.GetAllBalances(suite.Ctx, bondedModAddr)

			shares := d("1000")

			_, err = suite.Keeper.TransferDelegation(suite.Ctx, valAddr, fromDelegator, toDelegator, shares)
			suite.Require().NoError(err)

			// Transferring a delegation should move shares, and leave the validator and pool balances the same.

			suite.DelegationSharesEqual(valAddr, fromDelegator, fromDelegationShares.Sub(shares))
			suite.DelegationSharesEqual(valAddr, toDelegator, shares) // also creates new delegation

			validatorAfter, err := suite.StakingKeeper.GetValidator(suite.Ctx, valAddr)
			suite.Require().NoError(err)
			suite.Equal(validator.GetTokens(), validatorAfter.GetTokens())
			suite.Equal(validator.GetDelegatorShares(), validatorAfter.GetDelegatorShares())
			suite.Equal(validator.GetStatus(), validatorAfter.GetStatus())

			suite.AccountBalanceEqual(notBondedModAddr, notBondedBalance)
			suite.AccountBalanceEqual(bondedModAddr, bondedBalance)
		})
	}
}

func (suite *KeeperTestSuite) TestTransferDelegation_Shares() {
	_, addrs := testutil.GeneratePrivKeyAddressPairs(5)
	valAccAddr, fromDelegator, toDelegator := addrs[0], addrs[1], addrs[2]
	valAddr := sdk.ValAddress(valAccAddr)

	initialBalance := i(1e12)

	testCases := []struct {
		name              string
		createDelegations func() (fromDelegatorShares, toDelegatorShares sdkmath.LegacyDec, err error)
		shares            sdkmath.LegacyDec
		expectReceived    sdkmath.LegacyDec
		expectedErr       error
	}{
		{
			name: "negative shares cannot be transferred",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				// Run end blocker to update validator state to bonded.
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return fromDelegationShares, sdkmath.LegacyZeroDec(), nil
			},
			shares:      d("-1.0"),
			expectedErr: types.ErrUntransferableShares,
		},
		{
			name: "nil shares cannot be transferred",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return fromDelegationShares, sdkmath.LegacyZeroDec(), nil
			},
			shares:      sdkmath.LegacyDec{},
			expectedErr: types.ErrUntransferableShares,
		},
		{
			name: "0 shares cannot be transferred",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				toDelegationShares := suite.CreateDelegation(valAddr, toDelegator, i(2e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return fromDelegationShares, toDelegationShares, nil
			},
			shares:      sdkmath.LegacyZeroDec(),
			expectedErr: types.ErrUntransferableShares,
		},
		{
			name: "all shares can be transferred",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				toDelegationShares := suite.CreateDelegation(valAddr, toDelegator, i(2e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return fromDelegationShares, toDelegationShares, nil
			},
			shares:         d("1000000000.0"),
			expectReceived: d("1000000000.0"),
		},
		{
			name: "excess shares cannot be transferred",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return fromDelegationShares, sdkmath.LegacyZeroDec(), nil
			},
			shares:      d("1000000000.000000000000000001"),
			expectedErr: stakingtypes.ErrNotEnoughDelegationShares,
		},
		{
			name: "shares can be transferred to a non existent delegation",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return fromDelegationShares, sdkmath.LegacyZeroDec(), nil
			},
			shares:         d("500000000.0"),
			expectReceived: d("500000000.0"),
		},
		{
			name: "shares cannot be transferred from a non existent delegation",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), nil
			},
			shares:      d("500000000.0"),
			expectedErr: types.ErrNoDelegatorForAddress,
		},
		{
			name: "slashed validator shares can be transferred",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)

				suite.SlashValidator(valAddr, d("0.05"))

				return fromDelegationShares, sdkmath.LegacyZeroDec(), nil
			},
			shares:         d("500000000.0"),
			expectReceived: d("500000000.0"),
		},
		{
			name: "zero shares received when transfer < 1 token",
			createDelegations: func() (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
				suite.CreateNewUnbondedValidator(valAddr, i(1e9))
				fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(1e9))
				toDelegationShares := suite.CreateDelegation(valAddr, toDelegator, i(1e9))
				suite.StakingKeeper.EndBlocker(suite.Ctx)
				// make 1 share worth more than 1 token
				suite.SlashValidator(valAddr, d("0.05"))

				return fromDelegationShares, toDelegationShares, nil
			},
			shares:         d("1.0"), // send 1 share (truncates to zero tokens)
			expectReceived: d("0.0"),
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			suite.CreateAccountWithAddress(valAccAddr, suite.NewBondCoins(initialBalance))
			suite.CreateAccountWithAddress(fromDelegator, suite.NewBondCoins(initialBalance))
			suite.CreateAccountWithAddress(toDelegator, suite.NewBondCoins(initialBalance))

			fromDelegationShares, toDelegationShares, err := tc.createDelegations()
			suite.Require().NoError(err)
			validator, err := suite.StakingKeeper.GetValidator(suite.Ctx, valAddr)
			suite.Require().NoError(err)

			_, err = suite.Keeper.TransferDelegation(suite.Ctx, valAddr, fromDelegator, toDelegator, tc.shares)

			if tc.expectedErr != nil {
				suite.ErrorIs(err, tc.expectedErr)
				return
			}

			suite.NoError(err)
			suite.DelegationSharesEqual(valAddr, fromDelegator, fromDelegationShares.Sub(tc.shares))
			suite.DelegationSharesEqual(valAddr, toDelegator, toDelegationShares.Add(tc.expectReceived))

			validatorAfter, err := suite.StakingKeeper.GetValidator(suite.Ctx, valAddr)
			suite.Require().NoError(err)
			// total tokens should not change
			suite.Equal(validator.GetTokens(), validatorAfter.GetTokens())
			// but total shares can differ
			suite.Equal(
				validator.GetDelegatorShares().Sub(tc.shares).Add(tc.expectReceived),
				validatorAfter.GetDelegatorShares(),
			)
		})
	}
}

func (suite *KeeperTestSuite) TestTransferDelegation_RedelegationsForbidden() {
	_, addrs := testutil.GeneratePrivKeyAddressPairs(4)
	val1AccAddr, val2AccAddr, fromDelegator, toDelegator := addrs[0], addrs[1], addrs[2], addrs[3]
	val1Addr := sdk.ValAddress(val1AccAddr)
	val2Addr := sdk.ValAddress(val2AccAddr)

	initialBalance := i(1e12)

	suite.CreateAccountWithAddress(val1AccAddr, suite.NewBondCoins(initialBalance))
	suite.CreateAccountWithAddress(val2AccAddr, suite.NewBondCoins(initialBalance))
	suite.CreateAccountWithAddress(fromDelegator, suite.NewBondCoins(initialBalance))

	// create bonded validator 1 with a delegation
	suite.CreateNewUnbondedValidator(val1Addr, i(1e9))
	fromDelegationShares := suite.CreateDelegation(val1Addr, fromDelegator, i(1e9))
	suite.StakingKeeper.EndBlocker(suite.Ctx)

	// create validator 2 and redelegate to it
	suite.CreateNewUnbondedValidator(val2Addr, i(1e9))
	suite.CreateRedelegation(fromDelegator, val1Addr, val2Addr, i(1e9))
	suite.StakingKeeper.EndBlocker(suite.Ctx)

	_, err := suite.Keeper.TransferDelegation(suite.Ctx, val2Addr, fromDelegator, toDelegator, fromDelegationShares)
	suite.ErrorIs(err, types.ErrRedelegationsNotCompleted)
	suite.DelegationSharesEqual(val2Addr, fromDelegator, fromDelegationShares)
	suite.DelegationSharesEqual(val2Addr, toDelegator, sdkmath.LegacyZeroDec())
}

func (suite *KeeperTestSuite) TestTransferDelegation_CompliesWithMinSelfDelegation() {
	_, addrs := testutil.GeneratePrivKeyAddressPairs(4)
	valAccAddr, toDelegator := addrs[0], addrs[1]
	valAddr := sdk.ValAddress(valAccAddr)

	suite.CreateAccountWithAddress(valAccAddr, suite.NewBondCoins(i(1e12)))

	// create bonded validator with minimum delegated
	minSelfDelegation := i(1e9)
	delegation := suite.NewBondCoin(i(1e9))
	msg, err := stakingtypes.NewMsgCreateValidator(
		valAddr.String(),
		ed25519.GenPrivKey().PubKey(),
		delegation,
		stakingtypes.Description{
			Moniker:         "test-moniker",
			Identity:        "test-identity",
			Website:         "https://www.google.com/",
			SecurityContact: "sunrise17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9jfksztgw5uh69wac2pgs06edvm",
			Details:         "test-details",
		},
		stakingtypes.NewCommissionRates(sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec()),
		minSelfDelegation,
	)
	suite.Require().NoError(err)

	msgServer := stakingkeeper.NewMsgServerImpl(&suite.StakingKeeper)
	_, err = msgServer.CreateValidator(sdk.WrapSDKContext(suite.Ctx), msg)
	suite.Require().NoError(err)
	suite.StakingKeeper.EndBlocker(suite.Ctx)

	_, err = suite.Keeper.TransferDelegation(suite.Ctx, valAddr, valAccAddr, toDelegator, d("0.000000000000000001"))
	suite.ErrorIs(err, types.ErrSelfDelegationBelowMinimum)
	suite.DelegationSharesEqual(valAddr, valAccAddr, sdkmath.LegacyNewDecFromInt(delegation.Amount))
}

func (suite *KeeperTestSuite) TestTransferDelegation_CanTransferVested() {
	_, addrs := testutil.GeneratePrivKeyAddressPairs(4)
	valAccAddr, fromDelegator, toDelegator := addrs[0], addrs[1], addrs[2]
	valAddr := sdk.ValAddress(valAccAddr)

	suite.CreateAccountWithAddress(valAccAddr, suite.NewBondCoins(i(1e9)))
	suite.CreateVestingAccountWithAddress(fromDelegator, suite.NewBondCoins(i(2e9)), suite.NewBondCoins(i(1e9)))

	suite.CreateNewUnbondedValidator(valAddr, i(1e9))
	fromDelegationShares := suite.CreateDelegation(valAddr, fromDelegator, i(2e9))
	suite.StakingKeeper.EndBlocker(suite.Ctx)

	shares := d("1000000000.0")
	_, err := suite.Keeper.TransferDelegation(suite.Ctx, valAddr, fromDelegator, toDelegator, shares)
	suite.NoError(err)
	suite.DelegationSharesEqual(valAddr, fromDelegator, fromDelegationShares.Sub(shares))
	suite.DelegationSharesEqual(valAddr, toDelegator, shares)
}

func (suite *KeeperTestSuite) TestTransferDelegation_CannotTransferVesting() {
	_, addrs := testutil.GeneratePrivKeyAddressPairs(4)
	valAccAddr, fromDelegator, toDelegator := addrs[0], addrs[1], addrs[2]
	valAddr := sdk.ValAddress(valAccAddr)

	suite.CreateAccountWithAddress(valAccAddr, suite.NewBondCoins(i(1e9)))
	suite.CreateVestingAccountWithAddress(fromDelegator, suite.NewBondCoins(i(2e9)), suite.NewBondCoins(i(1e9)))

	suite.CreateNewUnbondedValidator(valAddr, i(1e9))
	suite.CreateDelegation(valAddr, fromDelegator, i(2e9))
	suite.StakingKeeper.EndBlocker(suite.Ctx)

	_, err := suite.Keeper.TransferDelegation(suite.Ctx, valAddr, fromDelegator, toDelegator, d("1000000001.0"))
	suite.ErrorIs(err, sdkerrors.ErrInsufficientFunds)
}
