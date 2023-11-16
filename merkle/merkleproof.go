package merkle

import (
	"reflect"
)

// Constants describing which side of the tree the proof node is on. If left_side then hash(proof_node || previous_hash) is computed, else hash(previous_hash || proof_node) is computed
const left_side uint = 0
const right_side uint = 1

type ProofNode struct {
	side uint
	hash *[]byte
}
type Proof struct {
	nodes     []ProofNode
	leaf_data []byte
}

func (proof Proof) Valid(root_hash []byte) bool {
	var leaf_hash = prefixed_hash(0, proof.leaf_data)
	var cur_hash = leaf_hash
	for i := 0; i < len(proof.nodes); i++ {
		if proof.nodes[i].hash == nil {
			cur_hash = prefixed_hash(1, cur_hash)
		} else if proof.nodes[i].side == left_side {
			cur_hash = prefixed_hash2(1, *proof.nodes[i].hash, cur_hash)
		} else {
			cur_hash = prefixed_hash2(1, cur_hash, *proof.nodes[i].hash)
		}
	}
	return reflect.DeepEqual(cur_hash, root_hash)
}
