package main

import "fmt"

// import "crypto/sha256"
import "blockchain/merkle"
import "blockchain/blockchain"
import "bytes"
import "encoding/gob"

func main() {
	var serialized bytes.Buffer
	data := []byte{1, 2, 3}
	gob.NewEncoder(&serialized).Encode(data)
	fmt.Println(serialized.Bytes())

	nodes := [][]byte{{97}, {97}}
	tree, _ := merkle.NewMerkleTree(nodes)
	for i := 0; i < len(tree.Nodes); i++ {
		fmt.Printf("%x\n", *tree.Nodes[i].Hash)
	}

	target := [32]byte{}
	for i := range target {
		target[i] = 0xff
	}
	target[0] = 0x00
	target[1] = 0x00
	target[2] = 0x00
	target[3] = 0xf0
	block := blockchain.NewBlockHeader(0, target[:], 0)
	fmt.Printf("Nonce %x\n", block.Nonce)
	block = blockchain.MineBlock(block, target)
	fmt.Printf("Nonce %x\n", block.Nonce)

	// foo, err := merkle.NewNode[[]byte](0)
	// fmt.Println(foo)
	// fmt.Println(nil)
	// fmt.Printf("%x", sha256.Sum256([]byte("")))
}
