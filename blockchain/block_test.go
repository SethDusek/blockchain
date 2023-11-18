package blockchain

import (
	"blockchain/schnorr"
	"math/big"
	"testing"
	"time"
)

func mine_n_blocks(t *testing.T, n int, block_chain *BlockChain, public_key schnorr.PublicKey) {
	for i := 0; i < n; i++ {
		start := time.Now().UnixMilli()
		block, _ := block_chain.NewBlockCandidate(public_key)
		t.Logf("Target %v\n", big.NewInt(0).SetBytes(block.Header.Target[:]))
		block.Header = MineBlock(block.Header)
		t.Logf("Finding block took %v ms\n", time.Now().UnixMilli()-start)
		if err := block_chain.AddBlock(*block); err != nil {
			t.Errorf("Error mining block %v #%v, %+v\n", err, i, block)
			return
		}
	}
}
func Test_Mining(t *testing.T) {
	block_chain := NewBlockChain()
	priv_key, _ := schnorr.NewPrivateKey()
	public_key := priv_key.PublicKey

	mine_n_blocks(t, 11, &block_chain, public_key)
	if count, err := block_chain.VerifyBlocks(); count != 11 || err != nil {
		t.Errorf("Error: mined valid blocks %v when should be 11 or error %v\n", count, err)
		return
	}

	new_chain := NewBlockChain()
	other_priv_key, _ := schnorr.NewPrivateKey()
	other_public_key := other_priv_key.PublicKey
	mine_n_blocks(t, 5, &new_chain, other_public_key)
	tree, _ := MakeTXMerkleTree(new_chain.Blocks[0].Transactions, 0)
	tree.PrettyPrint()

	if block_chain.AttemptOrphan(new_chain.Blocks) {
		panic("Orphaning longer chain should fail but succeeded")
	}
	if !new_chain.AttemptOrphan(block_chain.Blocks) {
		panic("Failed to build longest chain")
	}
	if count, err := new_chain.VerifyBlocks(); count != 11 || err != nil {
		t.Errorf("Error: mined valid blocks %v when should be 10 or error %v\n", count, err)
		return
	}

}
