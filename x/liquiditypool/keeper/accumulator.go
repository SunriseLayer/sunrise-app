package keeper

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/sunriselayer/sunrise/x/liquiditypool/types"

	"cosmossdk.io/math"
)

func (k Keeper) InitAccumulator(ctx context.Context, accumName string) error {
	store := k.storeService.OpenKVStore(ctx)
	hasKey, err := store.Has(types.FormatKeyAccumPrefix(accumName))
	if err != nil {
		return err
	}
	if hasKey {
		return errors.New("Accumulator with given name already exists in store")
	}

	return k.SetAccumulator(ctx, types.AccumulatorObject{
		Name:        accumName,
		AccumValue:  sdk.NewDecCoins(),
		TotalShares: math.LegacyZeroDec(),
	})
}

func (k Keeper) GetAccumulator(ctx context.Context, accumName string) (types.AccumulatorObject, error) {
	accumulator := types.AccumulatorObject{}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FormatKeyAccumPrefix(accumName))
	if err != nil {
		return types.AccumulatorObject{}, err
	}
	if bz == nil {
		return types.AccumulatorObject{}, types.ErrAccumDoesNotExist
	}

	err = proto.Unmarshal(bz, &accumulator)
	if err != nil {
		return types.AccumulatorObject{}, err
	}

	return accumulator, nil
}

func (k Keeper) SetAccumulator(ctx context.Context, accumulator types.AccumulatorObject) error {
	bz, err := proto.Marshal(&accumulator)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.FormatKeyAccumPrefix(accumulator.Name), bz)
}

func (k Keeper) AddToAccumulator(ctx context.Context, accumulator types.AccumulatorObject, amt sdk.DecCoins) {
	accumulator.AccumValue = accumulator.AccumValue.Add(amt...)
	err := k.SetAccumulator(ctx, accumulator)
	if err != nil {
		panic(err)
	}
}

func (k Keeper) GetAccumPosition(ctx context.Context, accumName, name string) (types.AccumRecord, error) {
	position := types.AccumRecord{}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FormatKeyAccumPositionPrefix(accumName, name))
	if err != nil {
		return types.AccumRecord{}, err
	}
	if bz == nil {
		return types.AccumRecord{}, types.ErrNoPosition
	}

	err = proto.Unmarshal(bz, &position)
	if err != nil {
		return types.AccumRecord{}, err
	}

	return position, nil
}

func (k Keeper) SetAccumPosition(ctx context.Context, accumName string, accumulatorValuePerShare sdk.DecCoins, index string, numShareUnits math.LegacyDec, unclaimedRewardsTotal sdk.DecCoins) {
	position := types.AccumRecord{
		NumShares:             numShareUnits,
		AccumValuePerShare:    accumulatorValuePerShare,
		UnclaimedRewardsTotal: unclaimedRewardsTotal,
	}
	bz, err := proto.Marshal(&position)
	if err != nil {
		panic(err)
	}
	store := k.storeService.OpenKVStore(ctx)
	err = store.Set(types.FormatKeyAccumPositionPrefix(accumName, index), bz)
	if err != nil {
		panic(err)
	}
}

func (k Keeper) NewPosition(ctx context.Context, accumName, name string, numShareUnits math.LegacyDec) error {
	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	return k.NewPositionIntervalAccumulation(ctx, accumName, name, numShareUnits, accumulator.AccumValue)
}

func (k Keeper) NewPositionIntervalAccumulation(ctx context.Context, accumName, name string, numShareUnits math.LegacyDec, intervalAccumulationPerShare sdk.DecCoins) error {
	k.SetAccumPosition(ctx, accumName, intervalAccumulationPerShare, name, numShareUnits, sdk.NewDecCoins())

	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}

	if accumulator.TotalShares.IsNil() {
		accumulator.TotalShares = math.LegacyZeroDec()
	}

	accumulator.TotalShares = accumulator.TotalShares.Add(numShareUnits)
	return k.SetAccumulator(ctx, accumulator)
}

func (k Keeper) AddToPosition(ctx context.Context, accumName, name string, newShares math.LegacyDec) error {
	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	return k.AddToPositionIntervalAccumulation(ctx, accumName, name, newShares, accumulator.AccumValue)
}

func (k Keeper) AddToPositionIntervalAccumulation(ctx context.Context, accumName, name string, newShares math.LegacyDec, intervalAccumulationPerShare sdk.DecCoins) error {
	if !newShares.IsPositive() {
		return errors.New("Adding non-positive number of shares to position")
	}

	position, err := k.GetAccumPosition(ctx, accumName, name)
	if err != nil {
		return err
	}

	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	unclaimedRewards := GetTotalRewards(accumulator, position)
	oldNumShares, err := k.GetAccumPositionSize(ctx, accumName, name)
	if err != nil {
		return err
	}

	k.SetAccumPosition(ctx, accumName, intervalAccumulationPerShare, name, oldNumShares.Add(newShares), unclaimedRewards)

	accumulator, err = k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	if accumulator.TotalShares.IsNil() {
		accumulator.TotalShares = math.LegacyZeroDec()
	}
	accumulator.TotalShares = accumulator.TotalShares.Add(newShares)
	return k.SetAccumulator(ctx, accumulator)
}

func (k Keeper) RemoveFromPosition(ctx context.Context, accumName, name string, numSharesToRemove math.LegacyDec) error {
	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	return k.RemoveFromPositionIntervalAccumulation(ctx, accumName, name, numSharesToRemove, accumulator.AccumValue)
}

func (k Keeper) RemoveFromPositionIntervalAccumulation(ctx context.Context, accumName, name string, numSharesToRemove math.LegacyDec, intervalAccumulationPerShare sdk.DecCoins) error {
	if !numSharesToRemove.IsPositive() {
		return fmt.Errorf("Removing non-positive shares (%s)", numSharesToRemove)
	}

	position, err := k.GetAccumPosition(ctx, accumName, name)
	if err != nil {
		return err
	}

	if numSharesToRemove.GT(position.NumShares) {
		return fmt.Errorf("Removing more shares (%s) than existing in the position (%s)", numSharesToRemove, position.NumShares)
	}

	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	unclaimedRewards := GetTotalRewards(accumulator, position)
	oldNumShares, err := k.GetAccumPositionSize(ctx, accumName, name)
	if err != nil {
		return err
	}

	k.SetAccumPosition(ctx, accumName, intervalAccumulationPerShare, name, oldNumShares.Sub(numSharesToRemove), unclaimedRewards)

	accumulator, err = k.GetAccumulator(ctx, accumName)
	if err != nil {
		return err
	}
	if accumulator.TotalShares.IsNil() {
		accumulator.TotalShares = math.LegacyZeroDec()
	}
	accumulator.TotalShares = accumulator.TotalShares.Sub(numSharesToRemove)
	return k.SetAccumulator(ctx, accumulator)
}

func (k Keeper) UpdatePositionIntervalAccumulation(ctx context.Context, accumName, name string, numShares math.LegacyDec, intervalAccumulationPerShare sdk.DecCoins) error {
	if numShares.IsZero() {
		return types.ErrZeroShares
	}

	if numShares.IsNegative() {
		return k.RemoveFromPositionIntervalAccumulation(ctx, accumName, name, numShares.Neg(), intervalAccumulationPerShare)
	}

	return k.AddToPositionIntervalAccumulation(ctx, accumName, name, numShares, intervalAccumulationPerShare)
}

func (k Keeper) SetPositionIntervalAccumulation(ctx context.Context, accumName, name string, intervalAccumulationPerShare sdk.DecCoins) error {
	position, err := k.GetAccumPosition(ctx, accumName, name)
	if err != nil {
		return err
	}

	k.SetAccumPosition(ctx, accumName, intervalAccumulationPerShare, name, position.NumShares, position.UnclaimedRewardsTotal)

	return nil
}

func (k Keeper) DeletePosition(ctx context.Context, accumName, positionName string) (sdk.DecCoins, error) {
	position, err := k.GetAccumPosition(ctx, accumName, positionName)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	remainingRewards, dust, err := k.ClaimRewards(ctx, accumName, positionName)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	store := k.storeService.OpenKVStore(ctx)
	err = store.Delete(types.FormatKeyAccumPositionPrefix(accumName, positionName))
	if err != nil {
		return sdk.DecCoins{}, err
	}

	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return sdk.DecCoins{}, err
	}
	accumulator.TotalShares.SubMut(position.NumShares)
	err = k.SetAccumulator(ctx, accumulator)
	if err != nil {
		return sdk.DecCoins{}, err
	}

	return sdk.NewDecCoinsFromCoins(remainingRewards...).Add(dust...), nil
}

func (k Keeper) deletePosition(ctx context.Context, accumName, positionName string) {
	store := k.storeService.OpenKVStore(ctx)
	err := store.Delete(types.FormatKeyAccumPositionPrefix(accumName, positionName))
	if err != nil {
		panic(err)
	}
}

func (k Keeper) GetAccumPositionSize(ctx context.Context, accumName, name string) (math.LegacyDec, error) {
	position, err := k.GetAccumPosition(ctx, accumName, name)
	if err != nil {
		return math.LegacyDec{}, err
	}

	return position.NumShares, nil
}

func (k Keeper) HasPosition(ctx context.Context, accumName, name string) bool {
	store := k.storeService.OpenKVStore(ctx)
	containsKey, err := store.Has(types.FormatKeyAccumPositionPrefix(accumName, name))
	if err != nil {
		panic(err)
	}
	return containsKey
}

func (k Keeper) ClaimRewards(ctx context.Context, accumName, positionName string) (sdk.Coins, sdk.DecCoins, error) {
	accumulator, err := k.GetAccumulator(ctx, accumName)
	if err != nil {
		return sdk.Coins{}, sdk.DecCoins{}, types.ErrNoPosition
	}

	position, err := k.GetAccumPosition(ctx, accumName, positionName)
	if err != nil {
		return sdk.Coins{}, sdk.DecCoins{}, types.ErrNoPosition
	}

	totalRewards := GetTotalRewards(accumulator, position)
	truncatedRewardsTotal, dust := totalRewards.TruncateDecimal()

	if position.NumShares.IsZero() {
		k.deletePosition(ctx, accumName, positionName)
	} else {
		k.SetAccumPosition(ctx, accumName, accumulator.AccumValue, positionName, position.NumShares, sdk.NewDecCoins())
	}

	return truncatedRewardsTotal, dust, nil
}

func (k Keeper) AddToUnclaimedRewards(ctx context.Context, accumName, positionName string, rewardsToAddTotal sdk.DecCoins) error {
	position, err := k.GetAccumPosition(ctx, accumName, positionName)
	if err != nil {
		return err
	}

	if rewardsToAddTotal.IsAnyNegative() {
		return types.ErrNegRewardAddition
	}

	k.SetAccumPosition(ctx, accumName, position.AccumValuePerShare, positionName, position.NumShares, position.UnclaimedRewardsTotal.Add(rewardsToAddTotal...))

	return nil
}

func GetTotalRewards(accumulator types.AccumulatorObject, position types.AccumRecord) sdk.DecCoins {
	totalRewards := position.UnclaimedRewardsTotal

	accumRewards := accumulator.AccumValue.Sub(position.AccumValuePerShare).MulDec(position.NumShares)
	totalRewards = totalRewards.Add(accumRewards...)

	return totalRewards
}
