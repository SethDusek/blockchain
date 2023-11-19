package blockchain

import (
	"blockchain/consensus"
	"blockchain/schnorr"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// An unspent transaction output.
type UTXO struct {
	Txout    [32]byte
	Outpoint uint32
}

// An output has a value and a challenge (a public key that must pass verification)
type Output struct {
	Value     uint64
	Challenge schnorr.PublicKey
}

type Transaction struct {
	Inputs  []UTXO
	Proofs  []schnorr.Signature
	Outputs []Output
}

// Generates the bytes that the signer must commit to
// The Commitment additionally commits to the block height if it's the coinbase transaction
// This is to prevent two coinbase transactions by the same miner having the same TXIDs, which would lead to some of the UTXOs being unspendable
func (tx Transaction) GenerateCommitment(block_height uint32) []byte {
	buf := new(bytes.Buffer)

	for _, input := range tx.Inputs {
		buf.Write(input.Txout[:])
		binary.Write(buf, binary.BigEndian, input.Outpoint)
	}
	// Coinbase transactions commit to block height as well
	if len(tx.Inputs) == 0 {
		binary.Write(buf, binary.BigEndian, block_height)
	}
	for _, output := range tx.Outputs {
		binary.Write(buf, binary.BigEndian, output.Value)
		binary.Write(buf, binary.BigEndian, output.Challenge)
	}
	return buf.Bytes()
}

func (tx Transaction) TXID(block_height uint32) [32]byte {
	hasher := sha256.New()
	hasher.Write(tx.GenerateCommitment(block_height))
	var txid [32]byte
	copy(txid[:], hasher.Sum(nil))
	return txid
}

// Create a new coinbase transaction for the miner's public key
func NewCoinbaseTransaction(public_key schnorr.PublicKey) Transaction {
	output := Output{consensus.BlockReward, public_key}
	return Transaction{[]UTXO{}, []schnorr.Signature{}, []Output{output}}
}

func (tx *Transaction) Sign(utxo_set *map[UTXO]Output, block_height uint32, private_key schnorr.PrivateKey) {
	committment := tx.GenerateCommitment(block_height)
	for i, input := range tx.Inputs {
		outpoint, ok := (*utxo_set)[input]
		if !ok || !outpoint.Challenge.Equal(private_key.PublicKey) {
			continue
		}
		tx.Proofs[i] = private_key.Sign(committment)
	}
}

func (tx Transaction) Verify(utxo_set map[UTXO]Output, is_coinbase bool, block_height uint32) bool {
	// Keep track of which inputs we've spent so double-spending is not allowed
	spent_inputs := map[UTXO]bool{}
	commitment := tx.GenerateCommitment(block_height)
	// A coinbase transaction must have no inputs
	if is_coinbase && len(tx.Inputs) != 0 {
		return false
	} else if !is_coinbase && len(tx.Inputs) == 0 { // Prevent non-coinbase transactions from having 0 inputs. This is an anti-spam mechanism
		return false
	}

	if len(tx.Proofs) < len(tx.Inputs) {
		fmt.Printf("Error: not enough proofs for inputs, %v inputs, %v proofs\n", len(tx.Inputs), len(tx.Proofs))
		return false
	}
	var input_value uint64 = 0
	for i, input := range tx.Inputs {
		output, ok := utxo_set[input]
		_, already_spent := spent_inputs[input]
		if !ok {
			fmt.Printf("Verification of tx %x failed, no input %x\n", tx.TXID(block_height), output)
			return false
		}
		if already_spent {
			fmt.Printf("Verification of tx %x failed, double-spend detected\n", tx.TXID(block_height))
		}
		spent_inputs[input] = true
		input_value += output.Value

		public_key := output.Challenge
		if !public_key.Verify(commitment, tx.Proofs[i]) {
			fmt.Printf("Signature verification of tx %x input[%v] failed\n", tx.TXID(block_height), i)
			return false
		}
	}
	var output_value uint64 = 0
	for _, output := range tx.Outputs {
		output_value += output.Value
	}

	if output_value > input_value {
		if !is_coinbase {
			fmt.Printf("Transaction creates more than it spends, input value %v, output value %v", input_value, output_value)
			return false
		} else if output_value-input_value <= consensus.BlockReward {
			return true
		} else {
			return false
		}
	}
	return true
}
