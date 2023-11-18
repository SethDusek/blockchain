package blockchain

import "encoding/json"

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
