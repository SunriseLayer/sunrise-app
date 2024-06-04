package keeper

import (
	fmt "fmt"

	db "github.com/cometbft/cometbft-db"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/math"

	"github.com/sunriselayer/sunrise/x/liquiditypool/swapstrategy"
	"github.com/sunriselayer/sunrise/x/liquiditypool/types"
)

type SwapState struct {
	amountSpecifiedRemaining        math.LegacyDec
	amountCalculated                math.LegacyDec
	sqrtPrice                       math.LegacyDec
	tick                            int64
	liquidity                       math.LegacyDec
	globalFeeGrowthPerUnitLiquidity math.LegacyDec
	globalFeeGrowth                 math.LegacyDec
	swapStrategy                    swapstrategy.SwapStrategy
}

type SwapResult struct {
	AmountIn  math.Int
	AmountOut math.Int
	Fees      math.LegacyDec
}

const swapNoProgressLimit = 100

func newSwapState(specifiedAmount math.Int, p types.Pool, strategy swapstrategy.SwapStrategy) SwapState {
	return SwapState{
		amountSpecifiedRemaining:        specifiedAmount.ToLegacyDec(),
		amountCalculated:                math.LegacyZeroDec(),
		sqrtPrice:                       p.CurrentSqrtPrice,
		tick:                            p.GetCurrentTick(),
		liquidity:                       p.CurrentTickLiquidity,
		globalFeeGrowthPerUnitLiquidity: math.LegacyZeroDec(),
		globalFeeGrowth:                 math.LegacyZeroDec(),
		swapStrategy:                    strategy,
	}
}

type SwapDetails struct {
	Sender   sdk.AccAddress
	TokenIn  sdk.Coin
	TokenOut sdk.Coin
}

type PoolUpdates struct {
	NewCurrentTick int64
	NewLiquidity   math.LegacyDec
	NewSqrtPrice   math.LegacyDec
}

func (ss *SwapState) updateFeeGrowthGlobal(feeChargeTotal math.LegacyDec) (math.LegacyDec, error) {
	feeChargeTotalScaled := feeChargeTotal
	ss.globalFeeGrowth = ss.globalFeeGrowth.Add(feeChargeTotal)

	if ss.liquidity.IsZero() {
		return math.LegacyZeroDec(), nil
	}

	feeRatesAccruedPerUnitOfLiquidityScaled := feeChargeTotalScaled.QuoTruncate(ss.liquidity)

	ss.globalFeeGrowthPerUnitLiquidity.AddMut(feeRatesAccruedPerUnitOfLiquidityScaled)

	return feeRatesAccruedPerUnitOfLiquidityScaled, nil
}

func (k Keeper) SwapExactAmountIn(
	ctx sdk.Context,
	sender sdk.AccAddress,
	pool types.Pool,
	tokenIn sdk.Coin,
	denomOut string) (amountOut math.Int, err error) {
	if tokenIn.Denom == denomOut {
		return math.Int{}, types.ErrDenomDuplication
	}

	baseForQuote := getBaseForQuote(tokenIn.Denom, pool.DenomBase)

	priceLimit := swapstrategy.GetPriceLimit(baseForQuote)
	tokenIn, tokenOut, _, err := k.swapOutAmtGivenIn(ctx, sender, pool, tokenIn, denomOut, pool.FeeRate, priceLimit)
	if err != nil {
		return math.Int{}, err
	}
	amountOut = tokenOut.Amount

	return amountOut, nil
}

func (k Keeper) SwapExactAmountOut(
	ctx sdk.Context,
	sender sdk.AccAddress,
	pool types.Pool,
	tokenOut sdk.Coin,
	denomIn string,
) (amountIn math.Int, err error) {
	if tokenOut.Denom == denomIn {
		return math.Int{}, types.ErrDenomDuplication
	}

	baseForQuote := getBaseForQuote(denomIn, pool.DenomBase)

	priceLimit := swapstrategy.GetPriceLimit(baseForQuote)
	tokenIn, tokenOut, _, err := k.swapInAmtGivenOut(ctx, sender, pool, tokenOut, denomIn, pool.FeeRate, priceLimit)
	if err != nil {
		return math.Int{}, err
	}
	amountIn = tokenIn.Amount

	return amountIn, nil
}

func (k Keeper) swapOutAmtGivenIn(
	ctx sdk.Context,
	sender sdk.AccAddress,
	pool types.Pool,
	tokenIn sdk.Coin,
	denomOut string,
	feeRate math.LegacyDec,
	priceLimit math.LegacyDec,
) (calcTokenIn, calcTokenOut sdk.Coin, poolUpdates PoolUpdates, err error) {
	swapResult, poolUpdates, err := k.computeOutAmtGivenIn(ctx, pool.GetId(), tokenIn, denomOut, feeRate, priceLimit, true)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, PoolUpdates{}, err
	}
	tokenIn = sdk.NewCoin(tokenIn.Denom, swapResult.AmountIn)
	tokenOut := sdk.NewCoin(denomOut, swapResult.AmountOut)

	if !tokenOut.Amount.IsPositive() {
		return sdk.Coin{}, sdk.Coin{}, PoolUpdates{}, types.ErrUnexpectedCalcAmount
	}

	if err := k.updatePoolForSwap(ctx, pool, SwapDetails{sender, tokenIn, tokenOut}, poolUpdates, swapResult.Fees); err != nil {
		return sdk.Coin{}, sdk.Coin{}, PoolUpdates{}, err
	}

	return tokenIn, tokenOut, poolUpdates, nil
}

func (k *Keeper) swapInAmtGivenOut(
	ctx sdk.Context,
	sender sdk.AccAddress,
	pool types.Pool,
	desiredTokenOut sdk.Coin,
	denomIn string,
	feeRate math.LegacyDec,
	priceLimit math.LegacyDec,
) (calcTokenIn, calcTokenOut sdk.Coin, poolUpdates PoolUpdates, err error) {
	swapResult, poolUpdates, err := k.computeInAmtGivenOut(ctx, desiredTokenOut, denomIn, feeRate, priceLimit, pool.GetId(), true)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, PoolUpdates{}, err
	}
	tokenIn := sdk.NewCoin(denomIn, swapResult.AmountIn)
	tokenOut := sdk.NewCoin(desiredTokenOut.Denom, swapResult.AmountOut)

	if !tokenIn.Amount.IsPositive() {
		return sdk.Coin{}, sdk.Coin{}, PoolUpdates{}, types.ErrUnexpectedCalcAmount
	}

	if err := k.updatePoolForSwap(ctx, pool, SwapDetails{sender, tokenIn, tokenOut}, poolUpdates, swapResult.Fees); err != nil {
		return sdk.Coin{}, sdk.Coin{}, PoolUpdates{}, err
	}

	return tokenIn, tokenOut, poolUpdates, nil
}

var unboundedPriceLimit = math.LegacyZeroDec()

func (k Keeper) CalculateResultExactAmountIn(
	ctx sdk.Context,
	pool types.Pool,
	tokenIn sdk.Coin,
	denomOut string,
) (tokenOut math.Int, err error) {
	cacheCtx, _ := ctx.CacheContext()
	swapResult, _, err := k.computeOutAmtGivenIn(cacheCtx, pool.Id, tokenIn, denomOut, pool.FeeRate, unboundedPriceLimit, false)
	if err != nil {
		return math.ZeroInt(), err
	}
	return swapResult.AmountOut, nil
}

func (k Keeper) CalculateResultExactAmountOut(
	ctx sdk.Context,
	pool types.Pool,
	tokenOut sdk.Coin,
	denomIn string,
) (math.Int, error) {
	cacheCtx, _ := ctx.CacheContext()
	swapResult, _, err := k.computeInAmtGivenOut(cacheCtx, tokenOut, denomIn, pool.FeeRate, unboundedPriceLimit, pool.Id, false)
	if err != nil {
		return math.ZeroInt(), err
	}
	return swapResult.AmountIn, nil
}

func (k Keeper) swapSetup(
	ctx sdk.Context,
	poolId uint64,
	denomIn string,
	denomOut string,
	getAccumulators bool,
) (pool types.Pool, feeAccum *AccumulatorObject, err error) {
	pool, err = k.getPoolForSwap(ctx, poolId)
	if err != nil {
		return pool, feeAccum, err
	}
	if err := checkDenomValidity(denomIn, denomOut, pool.DenomBase, pool.DenomQuote); err != nil {
		return pool, feeAccum, err
	}
	if getAccumulators {
		feeAccum, err = k.GetFeeAccumulator(ctx, poolId)
	}
	return pool, feeAccum, err
}

func iteratorToNextTickSqrtPriceTarget(nextInitTickIter db.Iterator, pool types.Pool, swapstrat swapstrategy.SwapStrategy) (int64, math.LegacyDec, math.LegacyDec, error) {
	if !nextInitTickIter.Valid() {
		return 0, math.LegacyDec{}, math.LegacyDec{}, types.ErrRanOutOfTicks
	}

	nextInitializedTick, err := types.TickIndexFromBytes(nextInitTickIter.Key())
	if err != nil {
		return 0, math.LegacyDec{}, math.LegacyDec{}, err
	}

	nextInitializedTickSqrtPrice, err := types.TickToSqrtPrice(nextInitializedTick, pool.TickParams)
	if err != nil {
		return 0, math.LegacyDec{}, math.LegacyDec{}, fmt.Errorf("could not convert next tick (%v) to nextSqrtPrice", nextInitializedTick)
	}

	sqrtPriceTarget := swapstrat.GetSqrtTargetPrice(nextInitializedTickSqrtPrice)
	return nextInitializedTick, nextInitializedTickSqrtPrice, sqrtPriceTarget, nil
}

func (k Keeper) computeOutAmtGivenIn(
	ctx sdk.Context,
	poolId uint64,
	minTokenIn sdk.Coin,
	denomOut string,
	feeRate math.LegacyDec,
	priceLimit math.LegacyDec,
	updateAccumulators bool,
) (swapResult SwapResult, poolUpdates PoolUpdates, err error) {
	p, feeAccumulator, err := k.swapSetup(ctx, poolId, minTokenIn.Denom, denomOut, updateAccumulators)
	if err != nil {
		return SwapResult{}, PoolUpdates{}, err
	}

	swapStrategy, sqrtPriceLimit, err := k.setupSwapStrategy(p, feeRate, minTokenIn.Denom, priceLimit)
	if err != nil {
		return SwapResult{}, PoolUpdates{}, err
	}

	swapState := newSwapState(minTokenIn.Amount, p, swapStrategy)

	nextInitTickIter := swapStrategy.NextTickIterator(ctx, poolId, swapState.tick)
	defer nextInitTickIter.Close()

	swapNoProgressIterationCount := 0
	for swapState.amountSpecifiedRemaining.IsPositive() && !swapState.sqrtPrice.Equal(sqrtPriceLimit) {
		sqrtPriceStart := swapState.sqrtPrice

		nextInitializedTick, nextInitializedTickSqrtPrice, sqrtPriceTarget, err := iteratorToNextTickSqrtPriceTarget(nextInitTickIter, p, swapStrategy)
		if err != nil {
			return SwapResult{}, PoolUpdates{}, err
		}

		computedSqrtPrice, amountIn, amountOut, feeCharge := swapStrategy.ComputeSwapWithinBucketOutGivenIn(
			swapState.sqrtPrice,
			sqrtPriceTarget,
			swapState.liquidity,
			swapState.amountSpecifiedRemaining,
		)

		if err := validateSwapProgressAndAmountConsumption(computedSqrtPrice, sqrtPriceStart, amountIn, amountOut); err != nil {
			return SwapResult{}, PoolUpdates{}, err
		}

		if updateAccumulators {
			_, err := swapState.updateFeeGrowthGlobal(feeCharge)
			if err != nil {
				return SwapResult{}, PoolUpdates{}, err
			}
		}

		swapState.sqrtPrice = computedSqrtPrice
		swapState.amountSpecifiedRemaining.SubMut(amountIn.Add(feeCharge))
		swapState.amountCalculated.AddMut(amountOut)
		if nextInitializedTickSqrtPrice.Equal(computedSqrtPrice) {
			swapState, err = k.swapCrossTickLogic(ctx, swapState, swapStrategy,
				nextInitializedTick, nextInitTickIter, p, feeAccumulator, minTokenIn.Denom, updateAccumulators)
			if err != nil {
				return SwapResult{}, PoolUpdates{}, err
			}
		} else if edgeCaseInequalityBasedOnSwapStrategy(swapStrategy.BaseForQuote(), nextInitializedTickSqrtPrice, computedSqrtPrice) {
			return SwapResult{}, PoolUpdates{}, types.ErrInvalidComputedSqrtPrice
		} else if !sqrtPriceStart.Equal(computedSqrtPrice) {
			newTick, err := types.CalculateSqrtPriceToTick(computedSqrtPrice, p.TickParams)
			if err != nil {
				return SwapResult{}, PoolUpdates{}, err
			}
			swapState.tick = newTick
		}

		if amountIn.IsZero() {
			if swapNoProgressIterationCount >= swapNoProgressLimit {
				return SwapResult{}, PoolUpdates{}, types.ErrRanOutOfIterations
			}
			swapNoProgressIterationCount++
		}
	}

	if swapState.amountSpecifiedRemaining.IsNegative() {
		return SwapResult{}, PoolUpdates{}, types.ErrOverChargeGivenIn
	}

	if updateAccumulators {
		feeGrowth := sdk.DecCoin{Denom: minTokenIn.Denom, Amount: swapState.globalFeeGrowthPerUnitLiquidity}
		feeAccumulator.AddToAccumulator(sdk.NewDecCoins(feeGrowth))
	}

	amountIn := minTokenIn.Amount.ToLegacyDec().SubMut(swapState.amountSpecifiedRemaining).Ceil().TruncateInt()
	amountOut := swapState.amountCalculated.TruncateInt()

	return SwapResult{
		AmountIn:  amountIn,
		AmountOut: amountOut,
		Fees:      swapState.globalFeeGrowth,
	}, PoolUpdates{swapState.tick, swapState.liquidity, swapState.sqrtPrice}, nil
}

func (k Keeper) computeInAmtGivenOut(
	ctx sdk.Context,
	desiredTokenOut sdk.Coin,
	denomIn string,
	feeRate math.LegacyDec,
	priceLimit math.LegacyDec,
	poolId uint64,
	updateAccumulators bool,
) (swapResult SwapResult, poolUpdates PoolUpdates, err error) {
	p, feeAccumulator, err := k.swapSetup(ctx, poolId, denomIn, desiredTokenOut.Denom, updateAccumulators)
	if err != nil {
		return SwapResult{}, PoolUpdates{}, err
	}

	swapStrategy, sqrtPriceLimit, err := k.setupSwapStrategy(p, feeRate, denomIn, priceLimit)
	if err != nil {
		return SwapResult{}, PoolUpdates{}, err
	}

	swapState := newSwapState(desiredTokenOut.Amount, p, swapStrategy)

	nextInitTickIter := swapStrategy.NextTickIterator(ctx, poolId, swapState.tick)
	defer nextInitTickIter.Close()

	swapNoProgressIterationCount := 0
	for swapState.amountSpecifiedRemaining.IsPositive() && !swapState.sqrtPrice.Equal(sqrtPriceLimit) {
		sqrtPriceStart := swapState.sqrtPrice

		nextInitializedTick, nextInitializedTickSqrtPrice, sqrtPriceTarget, err := iteratorToNextTickSqrtPriceTarget(nextInitTickIter, p, swapStrategy)
		if err != nil {
			return SwapResult{}, PoolUpdates{}, err
		}

		computedSqrtPrice, amountOut, amountIn, feeChargeTotal := swapStrategy.ComputeSwapWithinBucketInGivenOut(
			swapState.sqrtPrice,
			sqrtPriceTarget,
			swapState.liquidity,
			swapState.amountSpecifiedRemaining,
		)

		if err := validateSwapProgressAndAmountConsumption(computedSqrtPrice, sqrtPriceStart, amountIn, amountOut); err != nil {
			return SwapResult{}, PoolUpdates{}, err
		}

		if updateAccumulators {
			_, err := swapState.updateFeeGrowthGlobal(feeChargeTotal)
			if err != nil {
				return SwapResult{}, PoolUpdates{}, err
			}
		}

		swapState.sqrtPrice = computedSqrtPrice
		swapState.amountSpecifiedRemaining.SubMut(amountOut)
		swapState.amountCalculated.AddMut(amountIn.Add(feeChargeTotal))

		if nextInitializedTickSqrtPrice.Equal(computedSqrtPrice) {
			swapState, err = k.swapCrossTickLogic(ctx, swapState, swapStrategy,
				nextInitializedTick, nextInitTickIter, p, feeAccumulator, denomIn, updateAccumulators)
			if err != nil {
				return SwapResult{}, PoolUpdates{}, err
			}
		} else if edgeCaseInequalityBasedOnSwapStrategy(swapStrategy.BaseForQuote(), nextInitializedTickSqrtPrice, computedSqrtPrice) {
			return SwapResult{}, PoolUpdates{}, types.ErrInvalidComputedSqrtPrice
		} else if !sqrtPriceStart.Equal(computedSqrtPrice) {
			swapState.tick, err = types.CalculateSqrtPriceToTick(computedSqrtPrice, p.TickParams)
			if err != nil {
				return SwapResult{}, PoolUpdates{}, err
			}
		}

		if amountOut.IsZero() {
			if swapNoProgressIterationCount >= swapNoProgressLimit {
				return SwapResult{}, PoolUpdates{}, types.ErrRanOutOfIterations
			}
			swapNoProgressIterationCount++
		}
	}

	if swapState.amountSpecifiedRemaining.IsNegative() {
		return SwapResult{}, PoolUpdates{}, fmt.Errorf("over charged problem swap in given out by %s", swapState.amountSpecifiedRemaining)
	}

	if updateAccumulators {
		feeAccumulator.AddToAccumulator(sdk.NewDecCoins(sdk.NewDecCoinFromDec(denomIn, swapState.globalFeeGrowthPerUnitLiquidity)))
	}

	amountIn := swapState.amountCalculated.Ceil().TruncateInt()

	amountOut := desiredTokenOut.Amount.ToLegacyDec().SubMut(swapState.amountSpecifiedRemaining).TruncateInt()

	return SwapResult{
		AmountIn:  amountIn,
		AmountOut: amountOut,
		Fees:      swapState.globalFeeGrowth,
	}, PoolUpdates{swapState.tick, swapState.liquidity, swapState.sqrtPrice}, nil
}

func (k Keeper) swapCrossTickLogic(ctx sdk.Context,
	swapState SwapState, strategy swapstrategy.SwapStrategy,
	nextInitializedTick int64, nextTickIter db.Iterator,
	p types.Pool,
	feeAccum *AccumulatorObject,
	denomIn string,
	updateAccumulators bool,
) (SwapState, error) {
	nextInitializedTickInfo, err := ParseTickFromBz(nextTickIter.Value())
	if err != nil {
		return swapState, err
	}
	if updateAccumulators {
		// TODO: accumulator logic

		feeGrowth := sdk.DecCoin{Denom: denomIn, Amount: swapState.globalFeeGrowthPerUnitLiquidity}
		err := k.crossTick(ctx, p.Id, nextInitializedTick, &nextInitializedTickInfo, feeGrowth, feeAccum.GetValue())
		if err != nil {
			return swapState, err
		}
	}
	liquidityNet := nextInitializedTickInfo.LiquidityNet

	nextTickIter.Next()

	liquidityNet = swapState.swapStrategy.SetLiquidityDeltaSign(liquidityNet)
	swapState.liquidity.AddMut(liquidityNet)

	swapState.tick = strategy.NextTickAfterCrossing(nextInitializedTick)

	return swapState, nil
}

func (k Keeper) updatePoolForSwap(
	ctx sdk.Context,
	pool types.Pool,
	swapDetails SwapDetails,
	poolUpdates PoolUpdates,
	totalFees math.LegacyDec,
) error {
	feeRatesRoundedUp := sdk.NewCoin(swapDetails.TokenIn.Denom, totalFees.Ceil().TruncateInt())

	swapDetails.TokenIn.Amount = swapDetails.TokenIn.Amount.Sub(feeRatesRoundedUp.Amount)

	err := k.bankKeeper.SendCoins(ctx, swapDetails.Sender, pool.GetAddress(), sdk.Coins{swapDetails.TokenIn})
	if err != nil {
		return err
	}

	if !feeRatesRoundedUp.IsZero() {
		err = k.bankKeeper.SendCoins(ctx, swapDetails.Sender, pool.GetFeesAddress(), sdk.Coins{feeRatesRoundedUp})
		if err != nil {
			return err
		}
	}

	err = k.bankKeeper.SendCoins(ctx, pool.GetAddress(), swapDetails.Sender, sdk.Coins{swapDetails.TokenOut})
	if err != nil {
		return err
	}

	err = pool.ApplySwap(poolUpdates.NewLiquidity, poolUpdates.NewCurrentTick, poolUpdates.NewSqrtPrice)
	if err != nil {
		return fmt.Errorf("error applying swap: %w", err)
	}

	k.SetPool(ctx, pool)

	return err
}

func getBaseForQuote(inDenom, assetBase string) bool {
	return inDenom == assetBase
}

func checkDenomValidity(inDenom, outDenom, assetBase, assetQuote string) error {
	if outDenom != assetBase && outDenom != assetQuote {
		return types.ErrInvalidOutDenom
	}

	if inDenom != assetBase && inDenom != assetQuote {
		return types.ErrInvalidInDenom
	}
	if outDenom == inDenom {
		return types.ErrDenomDuplication
	}
	return nil
}

func (k Keeper) setupSwapStrategy(p types.Pool, feeRate math.LegacyDec, denomIn string, priceLimit math.LegacyDec) (strategy swapstrategy.SwapStrategy, sqrtPriceLimit math.LegacyDec, err error) {
	baseForQuote := getBaseForQuote(denomIn, p.DenomBase)

	// take provided price limit and turn into a sqrt price limit
	sqrtPriceLimit, err = swapstrategy.GetSqrtPriceLimit(priceLimit, baseForQuote)
	if err != nil {
		return strategy, math.LegacyDec{}, err
	}

	swapStrategy := swapstrategy.New(baseForQuote, sqrtPriceLimit, k.storeService, feeRate)

	// get current sqrt price
	curSqrtPrice := p.CurrentSqrtPrice
	if err := swapStrategy.ValidateSqrtPrice(sqrtPriceLimit, curSqrtPrice); err != nil {
		return strategy, math.LegacyDec{}, err
	}

	return swapStrategy, sqrtPriceLimit, nil
}

func (k Keeper) getPoolForSwap(ctx sdk.Context, poolId uint64) (types.Pool, error) {
	p, found := k.GetPool(ctx, poolId)
	if !found {
		return p, types.ErrPoolNotFound
	}
	hasPositionInPool := p.HasPosition(ctx)
	if !hasPositionInPool {
		return p, types.ErrEmptyLiquidity
	}
	return p, nil
}

func validateSwapProgressAndAmountConsumption(computedSqrtPrice, sqrtPriceStart math.LegacyDec, amountIn, amountOut math.LegacyDec) error {
	if computedSqrtPrice.Equal(sqrtPriceStart) && !(amountIn.IsZero() && amountOut.IsZero()) {
		return types.ErrNoSqrtPriceAfterSwap
	}
	return nil
}

func edgeCaseInequalityBasedOnSwapStrategy(isBaseForQuote bool, nextInitializedTickSqrtPrice, computedSqrtPrice math.LegacyDec) bool {
	if isBaseForQuote {
		return nextInitializedTickSqrtPrice.GT(computedSqrtPrice)
	}
	return nextInitializedTickSqrtPrice.LT(computedSqrtPrice)
}

func (k Keeper) ComputeMaxInAmtGivenMaxTicksCrossed(
	ctx sdk.Context,
	poolId uint64,
	denomIn string,
	maxTicksCrossed uint64,
) (maxTokenIn, resultingTokenOut sdk.Coin, err error) {
	cacheCtx, _ := ctx.CacheContext()

	p, err := k.getPoolForSwap(cacheCtx, poolId)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	if denomIn != p.DenomBase && denomIn != p.DenomQuote {
		return sdk.Coin{}, sdk.Coin{}, types.ErrInvalidInDenom
	}

	var denomOut string
	if denomIn == p.DenomBase {
		denomOut = p.DenomQuote
	} else {
		denomOut = p.DenomBase
	}

	swapStrategy, _, err := k.setupSwapStrategy(p, p.FeeRate, denomIn, math.LegacyZeroDec())
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	balance := k.bankKeeper.GetBalance(ctx, p.GetAddress(), denomOut)
	swapState := newSwapState(balance.Amount, p, swapStrategy)

	nextInitTickIter := swapStrategy.NextTickIterator(cacheCtx, poolId, swapState.tick)
	defer nextInitTickIter.Close()

	totalTokenOut := math.LegacyZeroDec()

	for i := uint64(0); i < maxTicksCrossed; i++ {
		if !nextInitTickIter.Valid() {
			break
		}

		nextInitializedTick, nextInitializedTickSqrtPrice, sqrtPriceTarget, err := iteratorToNextTickSqrtPriceTarget(nextInitTickIter, p, swapStrategy)
		if err != nil {
			return sdk.Coin{}, sdk.Coin{}, err
		}

		// Compute the swap
		computedSqrtPrice, amountOut, amountIn, feeChargeTotal := swapStrategy.ComputeSwapWithinBucketInGivenOut(
			swapState.sqrtPrice,
			sqrtPriceTarget,
			swapState.liquidity,
			swapState.amountSpecifiedRemaining,
		)

		swapState.sqrtPrice = computedSqrtPrice
		swapState.amountSpecifiedRemaining.SubMut(amountOut)
		swapState.amountCalculated.AddMut(amountIn.Add(feeChargeTotal))

		totalTokenOut = totalTokenOut.Add(amountOut)

		if nextInitializedTickSqrtPrice.Equal(computedSqrtPrice) {
			nextInitializedTickInfo, err := ParseTickFromBz(nextInitTickIter.Value())
			if err != nil {
				return sdk.Coin{}, sdk.Coin{}, err
			}
			liquidityNet := nextInitializedTickInfo.LiquidityNet

			nextInitTickIter.Next()

			liquidityNet = swapState.swapStrategy.SetLiquidityDeltaSign(liquidityNet)
			swapState.liquidity.AddMut(liquidityNet)

			swapState.tick = swapStrategy.NextTickAfterCrossing(nextInitializedTick)
		} else if edgeCaseInequalityBasedOnSwapStrategy(swapStrategy.BaseForQuote(), nextInitializedTickSqrtPrice, computedSqrtPrice) {
			return sdk.Coin{}, sdk.Coin{}, types.ErrNotEqualSqrtPrice
		} else if !swapState.sqrtPrice.Equal(computedSqrtPrice) {
			newTick, err := types.CalculateSqrtPriceToTick(computedSqrtPrice, p.TickParams)
			if err != nil {
				return sdk.Coin{}, sdk.Coin{}, err
			}
			swapState.tick = newTick
		}

		if amountOut.IsZero() {
			break
		}
	}

	maxAmt := swapState.amountCalculated.Ceil().TruncateInt()
	maxTokenIn = sdk.NewCoin(denomIn, maxAmt)
	resultingTokenOut = sdk.NewCoin(denomOut, totalTokenOut.TruncateInt())

	return maxTokenIn, resultingTokenOut, nil
}