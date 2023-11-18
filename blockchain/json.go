package blockchain

import (
	"blockchain/schnorr"
	"encoding/json"
)

type BlockJson struct {
	Blocks []Block
	Wallet *schnorr.PrivateKey
}

func (blockchain BlockChain) MarshalJSON() ([]byte, error) {
	// We can't serialize the UTXO set so we ignore it since it can be rebuilt anyway
	// TODO: to include or not to include mempool? Might be tricky to get right since we could be behind the longest chain and thus would have to remove transactions from mempool that are now invalid
	marshaled_fields := BlockJson{blockchain.Blocks, blockchain.Wallet}
	return json.Marshal(marshaled_fields)
}

func (blockchain *BlockChain) UnmarshalJSON(b []byte) error {
	blockchainjson := BlockJson{}
	if err := json.Unmarshal(b, &blockchainjson); err != nil {
		return err
	}
	blockchain.Blocks = blockchainjson.Blocks
	blockchain.Wallet = blockchainjson.Wallet
	return nil
}
