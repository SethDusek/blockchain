package blockchain

import (
	"blockchain/schnorr"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// An unspent transaction output.
type UTXO struct {
	txout     [32]byte
	outpoint  uint32
}

// An output has a value and a challenge (a public key that must pass verification)
type Output struct {
	value     uint64
	challenge schnorr.PublicKey
}

type Transaction struct {
	inputs  []UTXO
	proofs  []schnorr.Signature
	outputs []Output
}


// Generates the bytes that the signer must commit to
func (tx Transaction) GenerateComittment() []byte {
	buf := new(bytes.Buffer)

	for _, input := range tx.inputs {
		buf.Write(input.txout[:])
		binary.Write(buf, binary.BigEndian, input.outpoint)
	}
	for _, output := range tx.outputs {
		binary.Write(buf, binary.BigEndian, output.value)
		binary.Write(buf, binary.BigEndian, output.challenge)
	}
	return buf.Bytes()
}

func (tx Transaction) TXID() [32]byte {
	hasher := sha256.New()
	hasher.Write(tx.GenerateComittment())
	var txid [32]byte
	copy(txid[:], hasher.Sum(nil))
	return txid
}

func (tx Transaction) Verify(utxo_set map[UTXO]Output, is_coinbase bool) bool {
	committment := tx.GenerateComittment()
	// A coinbase transaction must have no inputs
	if is_coinbase && len(tx.inputs) != 0 {
		return false
	}

	if len(tx.proofs) < len(tx.inputs) {
		fmt.Printf("Error: not enough proofs for inputs, %v inputs, %v proofs", len(tx.inputs), len(tx.proofs))
		return false
	}
	var input_value uint64 = 0
	for i, input := range tx.inputs {
		output, ok := utxo_set[input]
		if !ok {
			fmt.Printf("Verification of tx %x failed, no input %x\n", tx.TXID(), output)
			return false
		}
		input_value+=output.value

		public_key := output.challenge
		if !public_key.Verify(committment, tx.proofs[i]) {
			fmt.Printf("Signature verification of tx %x input[%v] failed\n", tx.TXID(), i)
			return false
		}
	}
	var output_value uint64 = 0
	for _, output := range tx.outputs {
		output_value+=output.value
	}
	// Note we only check here that the output value does not exceed input value.
	// The actual block reward will be calculated and verified elsewhere
	if output_value > input_value && !is_coinbase {
		return false
	}

	return true
}
