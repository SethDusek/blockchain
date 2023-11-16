package merkle

import "testing"
import "reflect"

func TestEmptyTree(t *testing.T) {
	tree, err := NewMerkleTree(make([][]byte, 0))
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(tree.RootHash(), make([]byte, 32)) {
		t.Log(tree.RootHash())
		t.Errorf("Error, expected %x, found %x\n", make([]byte, 32), tree.RootHash())
	}
}

func TestOneElement(t *testing.T) {
	// Test a merkle tree with 1 leaf element
	nodes := [][]byte{{1}}
	tree, _ := NewMerkleTree(nodes)

	node, _ := new_node(nodes[0])
	manual_root_hash := *new_internal_node(node, Node{nil, nil}).Hash
	if !reflect.DeepEqual(tree.RootHash(), manual_root_hash) {
		t.Errorf("Error, expected %x, found %x\n", tree.RootHash(), manual_root_hash)
	}
}

func TestProof(t *testing.T) {
	nodes := make([][]byte, 0)
	for num_nodes := 1; num_nodes <= 1000; num_nodes++ {
		nodes = append(nodes, []byte{byte(num_nodes)})
		tree, _ := NewMerkleTree(nodes)
		for i := 0; i < len(nodes); i++ {
			proof := tree.ProveInclusion(nodes[i])
			if !proof.Valid(tree.RootHash()) {
				t.Errorf("Proof failed")
			}
		}
	}
}
