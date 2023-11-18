package blockchain

import (
	"blockchain/merkle"
	"blockchain/schnorr"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

type BlockHeader struct {
	Version    uint32
	PrevHash  [32]byte
	TXRootHash [32]byte
	Nonce      uint64
	Target     [32]byte
}

func NewBlockHeader(version uint32, prev_hash []byte, TXTree merkle.MerkleTree, nonce uint64, target [32]byte) BlockHeader {
	return BlockHeader{version, [32]byte(prev_hash), [32]byte(TXTree.RootHash()), nonce, target}
}
func (header BlockHeader) BlockHash() []byte {
	hasher := sha256.New()
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, header.Version)
	hasher.Write(buf.Bytes())
	buf.Reset()
	hasher.Write(header.PrevHash[:])
	hasher.Write(header.TXRootHash[:])
	binary.Write(buf, binary.LittleEndian, header.Nonce)
	hasher.Write(buf.Bytes())
	return hasher.Sum(nil)
}

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
		prev_hash = blockchain.Blocks[len(blockchain.Blocks) - 1].Header.BlockHash()
	}
	// TODO: difficulty targets
	var target [32]byte
	for i, _ := range target { target[i] = 0xff }
	block.Header = NewBlockHeader(1, prev_hash, *merkle_tree, 0, target)

	return &block, nil
}
func (blockchain BlockChain) MarshalJSON() ([]byte, error) {
	// We can't serialize the UTXO set so we ignore it since it can be rebuilt anyway
	// TODO: to include or not to include mempool? Might be tricky to get right since we could be behind the longest chain and thus would have to remove transactions from mempool that are now invalid
	return json.Marshal(blockchain.Blocks)
}

func (blockchain *BlockChain) UnmarshalJSON(b []byte) error {
	blocks := make([]Block, 0)
	if err := json.Unmarshal(b, &blocks); err != nil {
		return err
	}
	blockchain.Blocks = blocks
	return nil
}

// Add a new block to the blockchain. TODO: orphaning??
func (blockchain *BlockChain) AddBlock(block Block) error {
	if len(blockchain.Blocks) > 0 && block.Header.PrevHash != [32]byte(blockchain.Blocks[len(blockchain.Blocks)-1].Header.BlockHash()) {
		return errors.New("New Block does not point to tip of blockchain")
	}
	blockchain.Blocks = append(blockchain.Blocks, block)
	if !VerifyBlock(blockchain.Blocks, uint32(len(blockchain.Blocks) - 1), &blockchain.UTXOSet) {
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

// TODO: verify block hash matches difficulty target
func VerifyBlock(blocks []Block, block_idx uint32, utxo_set *map[UTXO]Output) bool {
	block := blocks[block_idx]
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
