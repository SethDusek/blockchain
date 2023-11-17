package blockchain

import "crypto/sha256"
import "encoding/binary"
import "bytes"
import "math/big"
type BlockHeader struct {
	version uint32
	prev_hash []byte
	Nonce uint64
}

func NewBlockHeader(version uint32, prev_hash []byte, nonce uint64) BlockHeader {
	return BlockHeader{version, prev_hash, nonce}
}
func (header BlockHeader) BlockHash() []byte {
	hasher := sha256.New()
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, header.version)
	hasher.Write(buf.Bytes())
	buf.Reset()
	hasher.Write(header.prev_hash)
	binary.Write(buf, binary.LittleEndian, header.Nonce)
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

func MineBlock(header BlockHeader, target [32]byte) BlockHeader {
	target_num := big.NewInt(0)
	target_num = target_num.SetBytes(target[:])
	for {
		cur_num := *big.NewInt(0)
		cur_num = *cur_num.SetBytes(header.BlockHash())
		if cur_num.Cmp(target_num) == -1 {
			return header
		}
		header.Nonce++
	}
	return header
}
