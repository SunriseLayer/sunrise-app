package types

const (
	// ModuleName defines the module name
	ModuleName = "da"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_da"

    
)

var (
	ParamsKey = []byte("p_da")
)



func KeyPrefix(p string) []byte {
    return []byte(p)
}
