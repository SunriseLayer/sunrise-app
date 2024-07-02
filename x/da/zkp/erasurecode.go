package zkp

import (
	"github.com/klauspost/reedsolomon"
)

func ErasureCode(blob []byte, shardCountHalf int) (shardSize uint64, shardCount int, shards [][]byte) {
	encoder, err := reedsolomon.New(shardCountHalf, shardCountHalf)
	if err != nil {
		panic(err)
	}

	shardCount = shardCountHalf * 2
	shards = make([][]byte, shardCount)

	length := len(blob)
	mod := length % shardCountHalf
	if mod != 0 {
		length += shardCountHalf - mod
	}
	shardSizeInt := length / shardCountHalf
	shardSize = uint64(shardSizeInt)

	for i := 0; i < shardCountHalf-1; i++ {
		shards[i] = make([]byte, shardSizeInt)
		copy(shards[i], blob[i*shardSizeInt:(i+1)*shardSizeInt])
	}
	i := shardCountHalf - 1
	shards[i] = make([]byte, shardSizeInt)
	copy(shards[i], blob[i*shardSizeInt:])

	for i := 0; i < shardCountHalf; i++ {
		shards[shardCountHalf+i] = make([]byte, shardSizeInt)
	}

	err = encoder.Encode(shards)
	if err != nil {
		panic(err)
	}

	return
}
