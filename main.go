package main

import (
	"blockchain/blockchain"
	"blockchain/schnorr"
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	//	"bufio"
	"encoding/json"
	"log"
	"os"
)

func load_blockchain() blockchain.BlockChain {
	if _, exists := os.Stat("blockchain.json"); os.IsNotExist(exists) {
		log.Printf("No blockchain stored on-disk, creating new")
		return blockchain.NewBlockChain()
	}
	file, err := os.ReadFile("blockchain.json")
	if err != nil {
		log.Fatal(err)
	}
	blockchain := blockchain.NewBlockChain()
	if err := json.Unmarshal(file, &blockchain); err != nil {
		log.Fatal("Error reading file from disk ", err)
	}
	if _, err := blockchain.VerifyBlocks(); err != nil {
		log.Fatal("Error verifying blocks from disk ", err)
	}
	return blockchain
}

func save_blockchain(blockchain *blockchain.BlockChain) {
	fmt.Println("Saving blockchain")
	marshaled, err := json.Marshal(*blockchain)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("blockchain.json", marshaled, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

// TODO: figure out how to communicate with goroutines properly
func miner_thread(candidate_block blockchain.Block, done chan<- blockchain.Block) {
	candidate_block.Header = blockchain.MineBlock(candidate_block.Header)
	done <- candidate_block
}

//	func mine_n_threads(block_chain blockchain.BlockChain, cores uint, done chan <- blockchain.Block) {
//		chans := make([]chan<-blockchain.Block, cores)
//		for channel := range chans {
//		}
//	}
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	block_chain := load_blockchain()
	defer save_blockchain(&block_chain)
	fmt.Println("here")
	for scanner.Scan() {
		command := strings.Split(scanner.Text(), " ")
		switch command[0] {
		case "printchain":
			block_chain.PrettyPrint()
		case "mineblock":
			channel := make(chan blockchain.Block)
			candidate, err := block_chain.NewBlockCandidate()
			if err != nil {
				log.Fatal(err)
			}
			start := time.Now().UnixMilli()
			go miner_thread(*candidate, channel)
			new_block := <-channel
			fmt.Printf("Found new block in %v ms\n", time.Now().UnixMilli()-start)
			block_chain.AddBlock(new_block)
		case "printbalances":
			balances := make(map[string]uint)
			for _, output := range block_chain.UTXOSet {
				balances[output.Challenge.ToAddress()] += uint(output.Value)
			}
			for pk, balance := range balances {
				fmt.Printf("%v: %v coins\n", pk, balance)
			}
		case "gennewaddress":
			private_key, err := schnorr.NewPrivateKey()
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(private_key.PublicKey.ToAddress())
		case "transfer":
			if len(command) != 3 {
				fmt.Println("Please enter transfer <destination address> amount")
			}
			public_key, err := schnorr.PublicKeyFromAddress(command[1])
			if err != nil {
				fmt.Println(err)
				continue
			}
			amount, err := strconv.Atoi(command[2])
			if err != nil {
				fmt.Println(err)
				continue
			}
			tx, err := block_chain.PayToPublicKey(*public_key, uint64(amount))
			fmt.Printf("Tx: %+v\n", tx)
			if err != nil {
				fmt.Println(err)
				continue
			}
			if err := block_chain.AddTXToMempool(*tx); err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Printf("Successful tx %+v, added to mempool", tx)

		case "changetransaction":
			fmt.Println("Enter block #: ")
			scanner.Scan()
			block_num, err := strconv.Atoi(scanner.Text())
			if err != nil {
				fmt.Println(err)
			}
			if block_num >= len(block_chain.Blocks) {
				fmt.Println("Invalid block")
			}
			fmt.Println("Enter block #: ")
			scanner.Scan()
			txnum, err := strconv.Atoi(scanner.Text())
			if err != nil {
				fmt.Println(err)
			}
			if txnum >= len(block_chain.Blocks[block_num].Transactions) {
				fmt.Println("Invalid tx")
			}
			fmt.Println("Decrementing output value by 1")
			block_chain.Blocks[block_num].Transactions[txnum].Outputs[0].Value-=1
			fmt.Println("Reverifying block-chain: ")
			prev_block_length := len(block_chain.Blocks)
			blocks, err := block_chain.VerifyBlocks()
			fmt.Printf("Validating blocks: #%v discarded, %v\n", int32(prev_block_length) - blocks, err)

		case "exit":
			return
		}
	}
	block_chain.PrettyPrint()

}
