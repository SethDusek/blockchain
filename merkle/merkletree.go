package merkle

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"math"
)

type Node struct {
	Hash *[]byte
	data *[]byte
}

func new_node(data []byte) (Node, error) {
	var serialized bytes.Buffer
	gob.NewEncoder(&serialized).Encode(data)

	hasher := sha256.New()
	// Prefix leaf nodes with a 0 to prevent second-preimage attacks
	hasher.Write([]byte{0})

	hasher.Write(data)
	hash := hasher.Sum(nil)
	return Node{&hash, &data}, nil
}

// Create a new Merkle Tree Node from 2 nodes
func new_internal_node(node1 Node, node2 Node) Node {
	hasher := sha256.New()
	if node1.Hash == nil && node2.Hash == nil {
		return Node{nil, nil}
	}
	if node1.Hash != nil {
		// Prefix internal nodes with a 1 to prevent second-preimage attacks
		hasher.Write([]byte{1})
		hasher.Write(*node1.Hash)
	}
	if node2.Hash != nil {
		hasher.Write(*node2.Hash)
	}
	hash := hasher.Sum(nil)
	return Node{&hash, nil}
}

func get_left(index uint) uint {
	return 2*index + 1
}
func get_right(index uint) uint {
	return 2*index + 2
}

// May return nil
func get_parent(index uint) *uint {
	if index == 0 {
		return nil
	}
	parent_idx := (index - 1) / 2
	return &parent_idx
}

func get_sibling(index uint) *uint {
	parent := get_parent(index)
	if parent == nil {
		return nil
	}
	left, right := get_left(*parent), get_right(*parent)
	// If left child is not equal to index, that means right child is sibling
	if left != index {
		return &left
	} else {
		return &right
	}
}

type MerkleTree struct {
	Nodes []Node
	// While our data is byte-arrays, we can't use them as keys. We convert the data to a string first which is a very cheap operation
	data_indices map[string]uint
}

func NewMerkleTree(elems [][]byte) (MerkleTree, error) {
	// For a full tree with l leaf nodes, there will be 2^n - 1 internal nodes, where n is such that 2^n >= l
	leaf_nodes := len(elems) + len(elems)%2
	internal_nodes := 0
	if leaf_nodes != 0 {
		internal_nodes = 1<<uint(math.Ceil(math.Log2(float64(leaf_nodes)))) - 1
	}
	nodes := make([]Node, internal_nodes+leaf_nodes)
	data_indices := make(map[string]uint)
	for i := 0; i < len(elems); i++ {
		node, err := new_node(elems[i])
		nodes[internal_nodes+i] = node
		if err != nil {
			return MerkleTree{}, err
		}
		data_indices[string(elems[i])] = uint(internal_nodes + i)
	}
	for i := len(nodes) - 2; i >= 1; i -= 2 {
		idx := uint(i)
		// Get_sibling will not panic here since we've padded nodes
		parent_node := new_internal_node(nodes[i], nodes[*get_sibling(idx)])
		nodes[*get_parent(idx)] = parent_node
	}
	return MerkleTree{nodes, data_indices}, nil
}

// Returns the Root Hash of the Merkle Tree. Returns an array of 0s if tree is empty
func (tree MerkleTree) RootHash() []byte {
	// For a tree with no nodes, return a 32-byte array of all zeros
	if len(tree.Nodes) == 0 {
		return make([]byte, 32)
	}
	return *tree.Nodes[0].Hash
}

func (tree MerkleTree) prove_inclusion(index uint) *Proof {
	proof_nodes := make([]ProofNode, 0)
	if index >= uint(len(tree.Nodes)) || tree.Nodes[index].data == nil {
		return nil
	}
	leaf_data := *tree.Nodes[index].data
	for {
		if get_sibling(index) == nil {
			break
		}
		sibling := *get_sibling(index)
		side := left_side
		if sibling == index+1 {
			side = right_side
		}
		proof_node := ProofNode{side, tree.Nodes[sibling].Hash}
		proof_nodes = append(proof_nodes, proof_node)

		if get_parent(index) == nil {
			break
		}
		index = *get_parent(index)
	}
	proof := Proof{proof_nodes, leaf_data}
	return &proof
}

func (tree MerkleTree) ProveInclusion(data []byte) *Proof {
	index, ok := tree.data_indices[string(data)]
	if !ok {
		return nil
	}
	return tree.prove_inclusion(index)
}
