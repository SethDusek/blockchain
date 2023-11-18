package blockchain

import "testing"
import "blockchain/schnorr"

func Test_Mining(t *testing.T) {
	block_chain := NewBlockChain()
	priv_key, _ := schnorr.NewPrivateKey()
	public_key := priv_key.PublicKey

	for i := 0; i < 10000; i++ {
		block, _ := block_chain.NewBlockCandidate(public_key)
		block.Header = MineBlock(block.Header)
		block_chain.AddBlock(*block)
	}
	if count, err := block_chain.VerifyBlocks(); count != 10000 || err != nil {
		t.Errorf("Error: mined valid blocks %v when should be 1000 or error %v\n", count, err)
	}

}
