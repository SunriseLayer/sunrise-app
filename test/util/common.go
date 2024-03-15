package util

import (
	"bytes"
	"testing"
	"time"

	blobstream "github.com/sunrise-zone/sunrise-app/x/blobstream/module"

	"github.com/sunrise-zone/sunrise-app/app"
	"github.com/sunrise-zone/sunrise-app/x/blobstream/keeper"
	bstypes "github.com/sunrise-zone/sunrise-app/x/blobstream/types"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/tx/signing"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ccodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
)

var (
	// ModuleBasics is a mock module basic manager for testing
	ModuleBasics = app.ModuleBasics()
	// TestingStakeParams is a set of staking params for testing
	TestingStakeParams = stakingtypes.Params{
		UnbondingTime:     100,
		MaxValidators:     10,
		MaxEntries:        10,
		HistoricalEntries: 10000,
		BondDenom:         "stake",
		MinCommissionRate: sdkmath.LegacyNewDecWithPrec(0, 0),
	}

	// ConsPrivKeys generate ed25519 ConsPrivKeys to be used for validator operator keys
	ConsPrivKeys = []ccrypto.PrivKey{
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
	}

	// ConsPubKeys holds the consensus public keys to be used for validator operator keys
	ConsPubKeys = []ccrypto.PubKey{
		ConsPrivKeys[0].PubKey(),
		ConsPrivKeys[1].PubKey(),
		ConsPrivKeys[2].PubKey(),
		ConsPrivKeys[3].PubKey(),
		ConsPrivKeys[4].PubKey(),
	}

	// AccPrivKeys generate secp256k1 pubkeys to be used for account pub keys
	AccPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}

	// AccPubKeys holds the pub keys for the account keys
	AccPubKeys = []ccrypto.PubKey{
		AccPrivKeys[0].PubKey(),
		AccPrivKeys[1].PubKey(),
		AccPrivKeys[2].PubKey(),
		AccPrivKeys[3].PubKey(),
		AccPrivKeys[4].PubKey(),
	}

	// AccAddrs holds the sdk.AccAddresses
	AccAddrs = []sdk.AccAddress{
		sdk.AccAddress(AccPubKeys[0].Address()),
		sdk.AccAddress(AccPubKeys[1].Address()),
		sdk.AccAddress(AccPubKeys[2].Address()),
		sdk.AccAddress(AccPubKeys[3].Address()),
		sdk.AccAddress(AccPubKeys[4].Address()),
	}

	// ValAddrs holds the sdk.ValAddresses
	ValAddrs = []sdk.ValAddress{
		sdk.ValAddress(AccPubKeys[0].Address()),
		sdk.ValAddress(AccPubKeys[1].Address()),
		sdk.ValAddress(AccPubKeys[2].Address()),
		sdk.ValAddress(AccPubKeys[3].Address()),
		sdk.ValAddress(AccPubKeys[4].Address()),
	}

	// EVMAddrs holds etheruem addresses
	EVMAddrs = initEVMAddrs(100)

	// InitTokens holds the number of tokens to initialize an account with
	InitTokens = sdk.TokensFromConsensusPower(110, sdk.DefaultPowerReduction)

	// InitCoins holds the number of coins to initialize an account with
	InitCoins = sdk.NewCoins(sdk.NewCoin(TestingStakeParams.BondDenom, InitTokens))

	// StakingAmount holds the staking power to start a validator with
	StakingAmount = sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)
)

func initEVMAddrs(count int) []gethcommon.Address {
	addresses := make([]gethcommon.Address, count)
	for i := 0; i < count; i++ {
		evmAddr := gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(i + 1)}, gethcommon.AddressLength))
		addresses[i] = evmAddr
	}
	return addresses
}

// TestInput stores the various keepers required to test Blobstream
type TestInput struct {
	BlobstreamKeeper keeper.Keeper
	AccountKeeper    authkeeper.AccountKeeper
	StakingKeeper    stakingkeeper.Keeper
	SlashingKeeper   slashingkeeper.Keeper
	DistKeeper       distrkeeper.Keeper
	BankKeeper       bankkeeper.Keeper
	Context          sdk.Context
	Marshaler        codec.Codec
	LegacyAmino      *codec.LegacyAmino
}

// CreateTestEnvWithoutBlobstreamKeysInit creates the keeper testing environment for Blobstream
func CreateTestEnvWithoutBlobstreamKeysInit(t *testing.T) TestInput {
	t.Helper()

	// Initialize store keys
	bsKey := storetypes.NewKVStoreKey(bstypes.StoreKey)
	keyAcc := storetypes.NewKVStoreKey(authtypes.StoreKey)
	keyStaking := storetypes.NewKVStoreKey(stakingtypes.StoreKey)
	keyBank := storetypes.NewKVStoreKey(banktypes.StoreKey)
	keyDistro := storetypes.NewKVStoreKey(distrtypes.StoreKey)
	keyParams := storetypes.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := storetypes.NewTransientStoreKey(paramstypes.TStoreKey)
	keySlashing := storetypes.NewKVStoreKey(slashingtypes.StoreKey)

	// Initialize memory database and mount stores on it
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	ms.MountStoreWithDB(bsKey, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyAcc, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyStaking, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyDistro, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, storetypes.StoreTypeTransient, db)
	ms.MountStoreWithDB(keySlashing, storetypes.StoreTypeIAVL, db)
	err := ms.LoadLatestVersion()
	require.Nil(t, err)

	// Create sdk.Context
	ctx := sdk.NewContext(ms, tmproto.Header{
		Version: tmversion.Consensus{
			Block: 0,
			App:   0,
		},
		ChainID: "test-app",
		Height:  1234567,
		Time:    time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		LastBlockId: tmproto.BlockID{
			Hash: []byte{},
			PartSetHeader: tmproto.PartSetHeader{
				Total: 0,
				Hash:  []byte{},
			},
		},
		LastCommitHash:     []byte{},
		DataHash:           []byte{},
		ValidatorsHash:     []byte{},
		NextValidatorsHash: []byte{},
		ConsensusHash:      []byte{},
		AppHash:            []byte{},
		LastResultsHash:    []byte{},
		EvidenceHash:       []byte{},
		ProposerAddress:    []byte{},
	}, false, log.NewNopLogger())

	cdc := MakeTestCodec()
	marshaler := MakeTestMarshaler()

	paramsKeeper := paramskeeper.NewKeeper(marshaler, cdc, keyParams, tkeyParams)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(bstypes.DefaultParamspace)
	paramsKeeper.Subspace(slashingtypes.ModuleName)

	// this is also used to initialize module accounts for all the map keys
	maccPerms := map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		bstypes.ModuleName:             {authtypes.Minter, authtypes.Burner},
	}

	authrity := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	accountKeeper := authkeeper.NewAccountKeeper(
		marshaler,
		runtime.NewKVStoreService(keyAcc), // target store
		authtypes.ProtoBaseAccount,        // prototype
		maccPerms,
		authcodec.NewBech32Codec(app.Bech32PrefixAccAddr),
		app.Bech32PrefixAccAddr,
		authrity,
	)

	blockedAddr := make(map[string]bool, len(maccPerms))
	for acc := range maccPerms {
		blockedAddr[authtypes.NewModuleAddress(acc).String()] = true
	}
	bankKeeper := bankkeeper.NewBaseKeeper(
		marshaler,
		runtime.NewKVStoreService(keyBank),
		accountKeeper,
		blockedAddr,
		authrity,
		log.NewNopLogger(),
	)
	bankKeeper.SetParams(
		ctx,
		banktypes.Params{
			SendEnabled:        []*banktypes.SendEnabled{},
			DefaultSendEnabled: true,
		},
	)

	stakingKeeper := stakingkeeper.NewKeeper(marshaler, runtime.NewKVStoreService(keyStaking), accountKeeper, bankKeeper, authrity, authcodec.NewBech32Codec(app.Bech32PrefixValAddr), authcodec.NewBech32Codec(app.Bech32PrefixConsAddr))
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	distKeeper := distrkeeper.NewKeeper(marshaler, runtime.NewKVStoreService(keyDistro), accountKeeper, bankKeeper, stakingKeeper, authtypes.FeeCollectorName, authrity)
	distKeeper.Params.Set(ctx, distrtypes.DefaultParams())

	// set genesis items required for distribution
	distKeeper.FeePool.Set(ctx, distrtypes.InitialFeePool())

	// total supply to track this
	totalSupply := sdk.NewCoins(sdk.NewInt64Coin("stake", 100000000))

	// set up initial accounts
	for name, perms := range maccPerms {
		macc := authtypes.NewEmptyModuleAccount(name, perms...)
		maccI := accountKeeper.NewAccount(ctx, macc).(sdk.ModuleAccountI)
		if name == stakingtypes.NotBondedPoolName {
			err = bankKeeper.MintCoins(ctx, bstypes.ModuleName, totalSupply)
			require.NoError(t, err)
			err = bankKeeper.SendCoinsFromModuleToModule(ctx, bstypes.ModuleName, maccI.GetName(), totalSupply)
			require.NoError(t, err)
		} else if name == distrtypes.ModuleName {
			// some big pot to pay out
			amt := sdk.NewCoins(sdk.NewInt64Coin("stake", 500000))
			err = bankKeeper.MintCoins(ctx, bstypes.ModuleName, amt)
			require.NoError(t, err)
			err = bankKeeper.SendCoinsFromModuleToModule(ctx, bstypes.ModuleName, maccI.GetName(), amt)
			require.NoError(t, err)
		}
		accountKeeper.SetModuleAccount(ctx, maccI)
	}

	stakeAddr := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)
	moduleAcct := accountKeeper.GetAccount(ctx, stakeAddr)
	require.NotNil(t, moduleAcct)

	legacyAmino := codec.NewLegacyAmino()
	slashingKeeper := slashingkeeper.NewKeeper(
		marshaler,
		legacyAmino,
		runtime.NewKVStoreService(keySlashing),
		stakingKeeper,
		authrity,
	)

	k := keeper.NewKeeper(marshaler, runtime.NewKVStoreService(bsKey), log.NewNopLogger(), authrity, stakingKeeper)
	testBlobstreamParams := bstypes.DefaultGenesis().Params
	k.SetParams(ctx, testBlobstreamParams)

	stakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			distKeeper.Hooks(),
			slashingKeeper.Hooks(),
			k.Hooks(),
		),
	)
	// emptyOpts := EmptyAppOptions{}
	// // encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)

	// testApp, _ := app.New(
	// 	log.NewNopLogger(), db, nil, true,
	// 	emptyOpts,
	// )

	// return TestInput{
	// 	BlobstreamKeeper: testApp.StreamKeeper,
	// 	AccountKeeper:    testApp.AccountKeeper,
	// 	BankKeeper:       testApp.BankKeeper,
	// 	StakingKeeper:    *testApp.StakingKeeper,
	// 	SlashingKeeper:   testApp.SlashingKeeper,
	// 	DistKeeper:       testApp.DistrKeeper,
	// 	Context:          testApp.NewContext(false),
	// 	Marshaler:        testApp.AppCodec(),
	// 	LegacyAmino:      testApp.LegacyAmino(),
	// }
	return TestInput{
		BlobstreamKeeper: k,
		AccountKeeper:    accountKeeper,
		BankKeeper:       bankKeeper,
		StakingKeeper:    *stakingKeeper,
		SlashingKeeper:   slashingKeeper,
		DistKeeper:       distKeeper,
		Context:          ctx,
		Marshaler:        marshaler,
		LegacyAmino:      cdc,
	}
}

// CreateTestEnv creates the keeper testing environment for Blobstream
func CreateTestEnv(t *testing.T) TestInput {
	input := CreateTestEnvWithoutBlobstreamKeysInit(t)
	input.BlobstreamKeeper.SetLatestAttestationNonce(input.Context, blobstream.InitialLatestAttestationNonce)
	input.BlobstreamKeeper.SetEarliestAvailableAttestationNonce(input.Context, blobstream.InitialEarliestAvailableAttestationNonce)
	return input
}

// MakeTestCodec creates a legacy amino codec for testing
func MakeTestCodec() *codec.LegacyAmino {
	cdc := codec.NewLegacyAmino()
	auth.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	bank.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	staking.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	distribution.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	sdk.RegisterLegacyAminoCodec(cdc)
	ccodec.RegisterCrypto(cdc)
	params.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	bstypes.RegisterLegacyAminoCodec(cdc)
	return cdc
}

// getSubspace returns a param subspace for a given module name.
func getSubspace(k paramskeeper.Keeper, moduleName string) paramstypes.Subspace {
	subspace, _ := k.GetSubspace(moduleName)
	return subspace
}

// MakeTestMarshaler creates a proto codec for use in testing
func MakeTestMarshaler() codec.Codec {
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          address.NewBech32Codec(app.Bech32PrefixAccAddr),
			ValidatorAddressCodec: address.NewBech32Codec(app.Bech32PrefixValAddr),
		},
	})
	std.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	bstypes.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}

// SetupFiveValChain does all the initialization for a 5 Validator chain using the keys here
func SetupFiveValChain(t *testing.T) (TestInput, sdk.Context) {
	t.Helper()
	input := CreateTestEnv(t)

	// Set the params for our modules
	input.StakingKeeper.SetParams(input.Context, TestingStakeParams)

	// Initialize each of the validators
	for i := range []int{0, 1, 2, 3, 4} {
		CreateValidator(t, input, AccAddrs[i], AccPubKeys[i], uint64(i), ValAddrs[i], ConsPubKeys[i], StakingAmount)
		RegisterEVMAddress(t, input, ValAddrs[i], EVMAddrs[i])
	}

	// Run the staking endblocker to ensure valset is correct in state
	input.StakingKeeper.EndBlocker(input.Context)

	// Return the test input
	return input, input.Context
}

func CreateValidator(
	t *testing.T,
	input TestInput,
	accAddr sdk.AccAddress,
	accPubKey ccrypto.PubKey,
	accountNumber uint64,
	valAddr sdk.ValAddress,
	consPubKey ccrypto.PubKey,
	stakingAmount sdkmath.Int,
) {
	// Initialize the account for the key
	acc := input.AccountKeeper.NewAccount(
		input.Context,
		authtypes.NewBaseAccount(accAddr, accPubKey, accountNumber, 0),
	)

	// Set the balance for the account
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, bstypes.ModuleName, InitCoins))
	err := input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, bstypes.ModuleName, acc.GetAddress(), InitCoins)
	require.NoError(t, err)

	// Set the account in state
	input.AccountKeeper.SetAccount(input.Context, acc)

	// Create a validator for that account using some tokens in the account
	// and the staking handler
	msgServer := stakingkeeper.NewMsgServerImpl(&input.StakingKeeper)
	_, err = msgServer.CreateValidator(input.Context, NewTestMsgCreateValidator(valAddr, consPubKey, stakingAmount))
	require.NoError(t, err)
}

func RegisterEVMAddress(
	t *testing.T,
	input TestInput,
	valAddr sdk.ValAddress,
	evmAddr gethcommon.Address,
) {
	bsMsgServer := keeper.NewMsgServerImpl(input.BlobstreamKeeper)
	registerMsg := bstypes.NewMsgRegisterEvmAddress(valAddr, evmAddr)
	_, err := bsMsgServer.RegisterEvmAddress(input.Context, registerMsg)
	require.NoError(t, err)
}

func NewTestMsgCreateValidator(
	address sdk.ValAddress,
	pubKey ccrypto.PubKey,
	amt sdkmath.Int,
) *stakingtypes.MsgCreateValidator {
	commission := stakingtypes.NewCommissionRates(sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec())
	out, err := stakingtypes.NewMsgCreateValidator(
		address.String(), pubKey, sdk.NewCoin("stake", amt),
		stakingtypes.Description{
			Moniker:         "test-moniker",
			Identity:        "test-identity",
			Website:         "https://www.google.com/",
			SecurityContact: "sunrise17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9jfksztgw5uh69wac2pgs06edvm",
			Details:         "test-details",
		}, commission, sdkmath.OneInt(),
	)
	if err != nil {
		panic(err)
	}
	return out
}

// SetupTestChain sets up a test environment with the provided validator voting weights
func SetupTestChain(t *testing.T, weights []uint64) (TestInput, sdk.Context) {
	t.Helper()
	input := CreateTestEnv(t)

	// Set the params for our modules
	TestingStakeParams.MaxValidators = 100
	input.StakingKeeper.SetParams(input.Context, TestingStakeParams)

	// Initialize each of the validators
	stakingMsgServer := stakingkeeper.NewMsgServerImpl(&input.StakingKeeper)
	bsMsgServer := keeper.NewMsgServerImpl(input.BlobstreamKeeper)
	for i, weight := range weights {
		consPrivKey := ed25519.GenPrivKey()
		consPubKey := consPrivKey.PubKey()
		valPrivKey := secp256k1.GenPrivKey()
		valPubKey := valPrivKey.PubKey()
		valAddr := sdk.ValAddress(valPubKey.Address())
		accAddr := sdk.AccAddress(valPubKey.Address())

		// Initialize the account for the key
		acc := input.AccountKeeper.NewAccount(
			input.Context,
			authtypes.NewBaseAccount(accAddr, valPubKey, uint64(i), 0),
		)

		// Set the balance for the account
		weightCoins := sdk.NewCoins(sdk.NewInt64Coin(TestingStakeParams.BondDenom, int64(weight)))
		require.NoError(t, input.BankKeeper.MintCoins(input.Context, bstypes.ModuleName, weightCoins))
		require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, bstypes.ModuleName, accAddr, weightCoins))

		// Set the account in state
		input.AccountKeeper.SetAccount(input.Context, acc)

		// Create a validator for that account using some of the tokens in the account
		// and the staking handler
		_, err := stakingMsgServer.CreateValidator(
			input.Context,
			NewTestMsgCreateValidator(valAddr, consPubKey, sdkmath.NewIntFromUint64(weight)),
		)
		require.NoError(t, err)

		registerMsg := bstypes.NewMsgRegisterEvmAddress(valAddr, EVMAddrs[i])
		_, err = bsMsgServer.RegisterEvmAddress(input.Context, registerMsg)
		require.NoError(t, err)

		// Run the staking endblocker to ensure valset is correct in state
		_, err = input.StakingKeeper.EndBlocker(input.Context)
		require.NoError(t, err)
	}

	// some inputs can cause the validator creation not to work, this checks that
	// everything was successful
	validators, _ := input.StakingKeeper.GetBondedValidatorsByPower(input.Context)
	require.Equal(t, len(weights), len(validators))

	// Return the test input
	return input, input.Context
}

func NewTestMsgUnDelegateValidator(address sdk.ValAddress, amt sdkmath.Int) *stakingtypes.MsgUndelegate {
	msg := stakingtypes.NewMsgUndelegate(sdk.AccAddress(address).String(), address.String(), sdk.NewCoin("stake", amt))
	return msg
}

// ExecuteBlobstreamHeights executes the end exclusive range of heights specified by beginHeight and endHeight
// along with the Blobstream abci.EndBlocker on each one of them.
// Returns the updated context with block height advanced to endHeight.
func ExecuteBlobstreamHeights(ctx sdk.Context, bsKeeper keeper.Keeper, beginHeight int64, endHeight int64) sdk.Context {
	for i := beginHeight; i < endHeight; i++ {
		ctx = ctx.WithBlockHeight(i)
		blobstream.EndBlocker(ctx, bsKeeper)
	}
	return ctx
}

// ExecuteBlobstreamHeightsWithTime executes the end exclusive range of heights specified by beginHeight and endHeight
// along with the Blobstream abci.EndBlocker on each one of them.
// Uses the interval to calculate the block header time.
func ExecuteBlobstreamHeightsWithTime(ctx sdk.Context, bsKeeper keeper.Keeper, beginHeight int64, endHeight int64, blockInterval time.Duration) sdk.Context {
	blockTime := ctx.BlockTime()
	for i := beginHeight; i < endHeight; i++ {
		ctx = ctx.WithBlockHeight(i).WithBlockTime(blockTime)
		blobstream.EndBlocker(ctx, bsKeeper)
		blockTime = blockTime.Add(blockInterval)
	}
	return ctx
}
