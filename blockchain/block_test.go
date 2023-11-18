package blockchain

import (
	"blockchain/schnorr"
	"math/big"
	"testing"
	"time"
)

func mine_n_blocks(t *testing.T, n int, block_chain *BlockChain) {
	for i := 0; i < n; i++ {
		start := time.Now().UnixMilli()
		block, _ := block_chain.NewBlockCandidate()
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

	mine_n_blocks(t, 11, &block_chain)
	if count, err := block_chain.VerifyBlocks(); count != 11 || err != nil {
		t.Errorf("Error: mined valid blocks %v when should be 11 or error %v\n", count, err)
		return
	}

	if utxos := block_chain.FindUTXOsByPublicKey(block_chain.Wallet.PublicKey); len(utxos) != 11 {
		t.Errorf("Miner should have 11 UTXOs, found %v", len(utxos))
		t.Logf("Miner address %v\n", block_chain.Wallet.PublicKey.ToAddress());
		for _, output := range block_chain.UTXOSet {
			t.Logf("%v %v\n", output.Challenge.ToAddress(), output.Value)
		}
		return
	}

	dest_private_key, _ := schnorr.NewPrivateKey()
	dest_public_key := dest_private_key.PublicKey
	// Attempt to pay 101 coins, this will create a change output
	tx, err := block_chain.PayToPublicKey(dest_public_key, 101)
	if err != nil {
		panic(err)
	}
	if !tx.Verify(block_chain.UTXOSet, false, uint32(len(block_chain.Blocks))) {
		panic(".")
	}
	t.Logf("Transaction: %+v\n", tx)
	if len(tx.Inputs) != 11 {
		t.Errorf("Expected 11 inputs, found %v\n", len(tx.Inputs))
	}
	if len(tx.Outputs) != 11 {
		t.Errorf("Expected 11 inputs, found %v\n", len(tx.Outputs))
	}

	new_chain := NewBlockChain()
	mine_n_blocks(t, 5, &new_chain)
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

	block_chain.PrettyPrint()

}
