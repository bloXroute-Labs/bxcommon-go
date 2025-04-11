package syncmap

import (
	"hash/maphash"

	"github.com/bloXroute-Labs/bxcommon-go/types"
)

// Hasher type of hasher functions
type Hasher[K comparable] func(maphash.Seed, K) uint64

// AccountIDHasher hasher function for AccountIDHasher key type.
// converts AccountIDHasher to string and returns Sum64 uint64
func AccountIDHasher(seed maphash.Seed, key types.AccountID) uint64 {
	return StringHasher(seed, string(key))
}

// StringHasher writes string hash and returns sum64
func StringHasher(seed maphash.Seed, key string) uint64 {
	return maphash.String(seed, key)
}
