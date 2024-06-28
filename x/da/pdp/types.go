package pdp

import (
	"fmt"
	"math/big"
)

type Public struct {
	P big.Int
	Q big.Int
	G big.Int
}

func (public *Public) Validate() error {
	r := new(big.Int).Exp(&public.G, &public.Q, &public.P).Cmp(big.NewInt(1))

	if r != 0 {
		return fmt.Errorf("It is required to satisfy g^q = 1 mod p")
	}

	return nil
}

type Proof struct {
	X         big.Int
	Y         big.Int
	TLargeBar big.Int
}

type Metadata struct {
	ShardSize  uint64
	ShardCount int
	ShardUris  []string
}