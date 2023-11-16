package merkle

import "crypto/sha256"

func prefixed_hash(prefix byte, data []byte) []byte {
	hasher := sha256.New()
	hasher.Write([]byte{prefix})
	hasher.Write(data)
	return hasher.Sum(nil)
}
func prefixed_hash2(prefix byte, data []byte, data2 []byte) []byte {
	hasher := sha256.New()
	hasher.Write([]byte{prefix})
	hasher.Write(data)
	hasher.Write(data2)
	return hasher.Sum(nil)
}
