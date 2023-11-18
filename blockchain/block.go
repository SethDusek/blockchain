package blockchain

import (
	"blockchain/consensus"
	"blockchain/merkle"
	"blockchain/schnorr"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"time"
)

type BlockHeader struct {
	Version    uint32
	PrevHash   [32]byte
	TXRootHash [32]byte
	Nonce      uint64
	Target     [32]byte
	// Unix timestamp in milliseconds
	Timestamp uint64
}

func NewBlockHeader(version uint32, prev_hash []byte, TXTree merkle.MerkleTree, nonce uint64, target [32]byte) BlockHeader {
	return BlockHeader{version, [32]byte(prev_hash), [32]byte(TXTree.RootHash()), nonce, target, uint64(time.Now().UnixMilli())}
}
func (header BlockHeader) BlockHash() []byte {
	hasher := sha256.New()
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, header.Version)
	binary.Write(buf, binary.LittleEndian, header.Nonce)
	binary.Write(buf, binary.LittleEndian, header.Timestamp)
	hasher.Write(buf.Bytes())
	hasher.Write(header.PrevHash[:])
	hasher.Write(header.TXRootHash[:])
	hasher.Write(header.Target[:])
	return hasher.Sum(nil)
}

// Mine a block, by finding nonce such that Hash(Block) < Target
func MineBlock(header BlockHeader) BlockHeader {
	target_num := big.NewInt(0).SetBytes(header.Target[:])
	for {
		cur_num := *big.NewInt(0)
		cur_num = *cur_num.SetBytes(header.BlockHash())
		if cur_num.Cmp(target_num) == -1 {
			return header
		}
		header.Nonce++
	}
}

type Block struct {
	Header       BlockHeader
	Transactions []Transaction
}

// A complete blockchain, including a list of blocks and the current UTXO set. Also includes a list of (valid) unconfirmed transactions
type BlockChain struct {
	Blocks  []Block
	UTXOSet map[UTXO]Output
	mempool []Transaction
}

func NewBlockChain() BlockChain {
	return BlockChain{make([]Block, 0), make(map[UTXO]Output), make([]Transaction, 0)}
}

func MakeTXMerkleTree(transactions []Transaction, block_height uint32) (*merkle.MerkleTree, error) {
	nodes := make([][]byte, 0, len(transactions))
	for _, tx := range transactions {
		txid := tx.TXID(block_height)
		nodes = append(nodes, txid[:])
	}
	merkle_tree, err := merkle.NewMerkleTree(nodes)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return &merkle_tree, nil
}

func (blockchain *BlockChain) NewBlockCandidate(miner_key schnorr.PublicKey) (*Block, error) {
	utxo_set := blockchain.UTXOSet
	transactions := make([]Transaction, 0)
	transactions = append(transactions, NewCoinbaseTransaction(miner_key))

	for _, tx := range blockchain.mempool {
		if !tx.Verify(utxo_set, false, uint32(len(blockchain.Blocks))) {
			continue
		}
		for _, input := range tx.Inputs {
			delete(utxo_set, input)
		}
		transactions = append(transactions, tx)
	}
	merkle_tree, err := MakeTXMerkleTree(transactions, uint32(len(blockchain.Blocks)))
	if err != nil {
		return nil, err
	}
	var block Block
	block.Transactions = transactions
	var prev_hash []byte = make([]byte, 32)
	if len(blockchain.Blocks) != 0 {
		prev_hash = blockchain.Blocks[len(blockchain.Blocks)-1].Header.BlockHash()
	}
	// TODO: difficulty targets
	var target [32]byte
	for i, _ := range target {
		target[i] = 0xff
	}
	target[0] = 0x00
	target[1] = 0x00
	if len(blockchain.Blocks) >= 2 {
		expected_target, err := Retarget(blockchain.Blocks[len(blockchain.Blocks)-2].Header.Timestamp,
			blockchain.Blocks[len(blockchain.Blocks)-1].Header.Timestamp,
			blockchain.Blocks[len(blockchain.Blocks)-1].Header.Target)
		if err != nil {
			return nil, err
		}
		target = expected_target
	}
	block.Header = NewBlockHeader(1, prev_hash, *merkle_tree, 0, target)

	return &block, nil
}

// Add a new block to the blockchain. TODO: orphaning??
func (blockchain *BlockChain) AddBlock(block Block) error {
	if len(blockchain.Blocks) > 0 && block.Header.PrevHash != [32]byte(blockchain.Blocks[len(blockchain.Blocks)-1].Header.BlockHash()) {
		return errors.New("New Block does not point to tip of blockchain")
	}
	blockchain.Blocks = append(blockchain.Blocks, block)
	if !VerifyBlock(blockchain.Blocks, uint32(len(blockchain.Blocks)-1), &blockchain.UTXOSet) {
		blockchain.Blocks = blockchain.Blocks[:len(blockchain.Blocks)-1]
		return errors.New("Verification failed")
	}
	// Update UTXO set
	for _, tx := range blockchain.Blocks[len(blockchain.Blocks)-1].Transactions {
		for _, input := range tx.Inputs {
			delete(blockchain.UTXOSet, input)
		}
		for outpoint, output := range tx.Outputs {
			utxo := UTXO{tx.TXID(uint32(len(blockchain.Blocks))), uint32(outpoint)}
			blockchain.UTXOSet[utxo] = output
		}
	}
	return nil
}

// Calculate new difficulty target
// Example: If the blocktime is 60 seconds, and a block is found in 30 seconds, then it appeared (30/60) = 0.5x faster. This will then halve the target, which will be the target for next block
// Similarly if a blocktime is 60 seconds and a block is found in 120 seconds, then it appeared (120/60) = 2x slower than it should. This will double the target
// We don't do any averaging, so difficulty is recalculated *every* block based on the last 2 blocks rather than over some epoch
// The maximum allowed difficulty difference from one block to the next is 20%
func Retarget(prev_timestamp uint64, cur_timestamp uint64, prev_target [32]byte) ([32]byte, error) {
	new_target := big.NewInt(0).SetBytes(prev_target[:])
	if cur_timestamp < prev_timestamp {
		return [32]byte{}, errors.New("New timestamp can't be < previous timestamp!")
	}
	cur_blocktime := (cur_timestamp - prev_timestamp) / 1000
	if cur_blocktime == 0 {
		cur_blocktime = 1
	}
	new_target = new_target.Mul(new_target, big.NewInt(int64(cur_blocktime)))
	new_target = new_target.Div(new_target, big.NewInt(int64(consensus.BlockTime)))
	// If we can't fit new target in 256 bits, simply ignore it for now
	if new_target.BitLen() > 256 {
		return prev_target, nil
	}
	prev_target_num := big.NewInt(0).SetBytes(prev_target[:])
	if new_target.Cmp(prev_target_num) == -1 {
		diff := big.NewInt(0).Sub(prev_target_num, new_target)
		fifth := big.NewInt(0).Div(prev_target_num, big.NewInt(5))
		// Calculate new difficulty to be 80% of previous difficulty
		if diff.Cmp(fifth) == 1 {
			new_target = prev_target_num.Sub(prev_target_num, fifth)
		}
	}
	var retargeted [32]byte
	copy(retargeted[32-len(new_target.Bytes()):], new_target.Bytes())
	return retargeted, nil
}

// TODO: verify block hash matches difficulty target
func VerifyBlock(blocks []Block, block_idx uint32, utxo_set *map[UTXO]Output) bool {
	block := blocks[block_idx]

	hash := big.NewInt(0).SetBytes(block.Header.BlockHash())
	target := big.NewInt(0).SetBytes(block.Header.Target[:])
	if hash.Cmp(target) != -1 {
		fmt.Printf("Block hash is not below target, hash: %v, target: %v\n", hash, target)
		return false
	}

	if len(blocks) > 2 {
		expected_target, err := Retarget(blocks[block_idx-2].Header.Timestamp, blocks[block_idx-1].Header.Timestamp, blocks[block_idx-1].Header.Target)
		if err != nil {
			fmt.Printf("Error calculating difficulty %v\n", err)
			return false
		}
		if expected_target != block.Header.Target {
			fmt.Printf("Error, target does not match. Expected target %v, actual target %v", big.NewInt(0).SetBytes(expected_target[:]), big.NewInt(0).SetBytes(block.Header.Target[:]))
			return false
		}
	}
	merkle_tree, err := MakeTXMerkleTree(block.Transactions, block_idx)
	if err != nil {
		return false
	}
	if !reflect.DeepEqual(merkle_tree.RootHash(), block.Header.TXRootHash[:]) {
		fmt.Printf("Merkle Tree equality failed, block header root hash: %x, actual root hash: %x\n", block.Header.TXRootHash, merkle_tree.RootHash())
		return false
	}
	// The genesis block must point to [0; 32]
	if block_idx != 0 && block.Header.PrevHash != [32]byte(blocks[block_idx-1].Header.BlockHash()) {
		fmt.Printf("Chain is not consistent! block %v points to %x but block %v is %x\n", block_idx, blocks[block_idx].Header.PrevHash, block_idx-1, blocks[block_idx-1].Header.BlockHash())
		return false
	} else if block_idx == 0 && block.Header.PrevHash != [32]byte{} {
		fmt.Printf("Genesis block's prev_hash must be 0")
		return false
	}
	for i, tx := range blocks[block_idx].Transactions {
		if !tx.Verify(*utxo_set, i == 0, block_idx) {
			fmt.Printf("Verifying tx %v failed\n", i)
			return false
		}
	}
	return true
}

// Verifies blocks from genesis to tip of chain. If it errors early, it will return the index of the last valid block (or -1 if none are valid) and also delete all the blocks from blockchain
func (blockchain *BlockChain) VerifyBlocks() (int32, error) {
	new_blockchain := NewBlockChain()
	copy(new_blockchain.mempool, blockchain.mempool)

	for i, block := range blockchain.Blocks {
		err := new_blockchain.AddBlock(block)
		if err != nil {
			fmt.Printf("Error verifying Block %v hash %v\n", i, block.Header.BlockHash())
			*blockchain = new_blockchain
			return int32(i) - 1, err
		}
	}
	*blockchain = new_blockchain
	return int32(len(blockchain.Blocks)), nil
}

// Returns index of block
func (blockchain *BlockChain) SearchBlockByHash(hash [32]byte) *int {
	for i, block := range blockchain.Blocks {
		if [32]byte(block.Header.BlockHash()) == hash {
			return &i
		}
	}
	return nil
}

// Attempts to orphan blocks with new longest chain
func (block_chain *BlockChain) AttemptOrphan(blocks []Block) bool {
	if len(blocks) == 0 {
		return false
	}
	cloned_chain := *block_chain
	start_idx := 0
	if blocks[0].Header.PrevHash == [32]byte{} {
		start_idx = 0
	} else {
		idx := block_chain.SearchBlockByHash(blocks[0].Header.PrevHash)
		if idx == nil {
			fmt.Printf("Could not find starting of new chain prevhash %x\n", blocks[0].Header.PrevHash)
			return false
		}
		start_idx = *idx
	}
	if start_idx+len(blocks) <= len(block_chain.Blocks) {
		fmt.Printf("New chain is not longer, new chain length %v, our chain length %v\n", start_idx+len(blocks), len(block_chain.Blocks))
		return false
	}
	cloned_chain.Blocks = make([]Block, 0)
	cloned_chain.UTXOSet = make(map[UTXO]Output, 0)
	for _, block := range block_chain.Blocks[:start_idx] {
		if err := cloned_chain.AddBlock(block); err != nil {
			fmt.Println(err)
			return false
		}
	}
	for _, block := range blocks {
		if err := cloned_chain.AddBlock(block); err != nil {
			fmt.Println(err)
			return false
		}
	}
	*block_chain = cloned_chain

	return true
}
