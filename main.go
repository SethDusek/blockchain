package main

import (
	"blockchain/blockchain"
	"blockchain/merkle"
	"blockchain/schnorr"
	"bytes"
	"encoding/gob" // import "crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
)

func main() {
	var serialized bytes.Buffer
	data := []byte{1, 2, 3}
	gob.NewEncoder(&serialized).Encode(data)
	fmt.Println(serialized.Bytes())

	nodes := [][]byte{{97}, {97}}
	tree, _ := merkle.NewMerkleTree(nodes)
	for i := 0; i < len(tree.Nodes); i++ {
		//fmt.Printf("%x\n", *tree.Nodes[i].Hash)
	}

	target := [32]byte{}
	for i := range target {
		target[i] = 0xff
	}

	target_num := big.NewInt(0).SetBytes(target[:])
	fmt.Println(target_num, target_num.BitLen())
	new_target, err := blockchain.Retarget(0, 120, target)
	new_target_num := big.NewInt(0).SetBytes(new_target[:])
	fmt.Println(new_target_num, new_target_num.BitLen())

	return
	var block_chain blockchain.BlockChain = blockchain.NewBlockChain()

	priv_key, _ := schnorr.NewPrivateKey()
	pub_key := priv_key.PublicKey

	block_candidate, err := block_chain.NewBlockCandidate(pub_key)
	fmt.Println(err)
	jason, _ := json.Marshal(block_candidate)
	fmt.Printf("block json %s\n", jason)

	block_candidate.Header = blockchain.MineBlock(block_candidate.Header)
	block_chain.AddBlock(*block_candidate)
	block_candidate, err = block_chain.NewBlockCandidate(pub_key)
	block_candidate.Header = blockchain.MineBlock(block_candidate.Header)
	block_chain.AddBlock(*block_candidate)

	jason, _ = json.Marshal(block_chain)
	fmt.Printf("Blocks: %s\n", jason)
	for key, val := range block_chain.UTXOSet {
		fmt.Printf("UTXO: %v output: %v", key, val)
	}



	// block := blockchain.NewBlockHeader(0, target[:], 0)
	// fmt.Printf("Nonce %x\n", block.Nonce)
	// block = blockchain.MineBlock(block, target)
	// fmt.Printf("Nonce %x\n", block.Nonce)

	// foo, err := merkle.NewNode[[]byte](0)
	// fmt.Println(foo)
	// fmt.Println(nil)
	// fmt.Printf("%x", sha256.Sum256([]byte("")))
}
