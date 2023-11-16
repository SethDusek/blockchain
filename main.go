package main

import "fmt"

// import "crypto/sha256"
import "blockchain/merkle"
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

	// foo, err := merkle.NewNode[[]byte](0)
	// fmt.Println(foo)
	// fmt.Println(nil)
	// fmt.Printf("%x", sha256.Sum256([]byte("")))
}
