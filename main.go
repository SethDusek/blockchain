package main

import (
	"blockchain/blockchain"
	"blockchain/p2p"
	"blockchain/schnorr"
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"

	"time"

	//	"bufio"
	"encoding/hex"
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

//	func mine_n_threads(block_chain blockchain.BlockChain, cores uint, done chan <- blockchain.Block) {
//		chans := make([]chan<-blockchain.Block, cores)
//		for channel := range chans {
//		}
//	}

func start_tcp_server(block_chain *blockchain.BlockChain, port int) *p2p.Node {
	var listener *net.Listener
	for i := 0; i <= 30; i++ {
		server, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			listener = &server
			break
		} else {
			log.Printf("Error listening on port %v, err: %v \n", port, err)
		}
		port++
	}
	if listener == nil {
		log.Fatal("Tried 10 ports, could not listen on any!")
	}
	return p2p.NewNode(*listener, block_chain)
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	block_chain := load_blockchain()
	defer save_blockchain(&block_chain)

	// Start P2P network
	node := start_tcp_server(&block_chain, 8284)
	fmt.Println("Blockchain instance started")
	for scanner.Scan() {
		command := strings.Split(scanner.Text(), " ")
		switch command[0] {
		case "startminer":
			node.StartMiner()
		case "connect":
			if len(command) != 2 {
				fmt.Println("Please provide address:port")
			} else {
				err := node.Connect(command[1], true)
				if err != nil {
					log.Println(err)
					continue
				}
				node.PrintConnectedPeers()
				go func() {
					node.Sync()
					//node.StartMiner()
				}()
			}

		case "printchain":
			block_chain.PrettyPrint()
		case "mineblock":
			candidate, err := block_chain.NewBlockCandidate()
			if err != nil {
				log.Fatal(err)
			}
			start := time.Now().UnixMilli()
			candidate.Header = blockchain.MineBlock(candidate.Header, nil, nil)
			fmt.Printf("Found new block in %v ms\n", time.Now().UnixMilli()-start)
			node.NewBlock(*candidate, "")
		case "printaddress":
			fmt.Println(block_chain.Wallet.PublicKey.ToAddress())
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
			if err := node.NewTransaction(*tx, ""); err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Printf("Successful tx %+x, added to mempool\n", tx.TXID(0))
		case "createtransactions":
			utxos := block_chain.FindUTXOsByPublicKey(block_chain.Wallet.PublicKey)
			fmt.Printf("Enter amount of transactions to create (max %v): ", len(utxos));
			scanner.Scan()
			count, err := strconv.Atoi(scanner.Text());
			if err != nil {
				fmt.Println(err);
				continue;
			}
			for i, utxo := range utxos {
				if i == count {
					break
				}
				transaction := blockchain.Transaction{Inputs: make([]blockchain.UTXO, 1), Proofs: make([]schnorr.Signature, 1), Outputs: make([]blockchain.Output, 1)}
				transaction.Inputs[0] = utxo.UTXO
				transaction.Outputs[0] = blockchain.Output{Value: utxo.Value, Challenge: block_chain.Wallet.PublicKey}
				transaction.Sign(&block_chain.UTXOSet, 0, *block_chain.Wallet)
				if err := node.NewTransaction(transaction, ""); err != nil {
					fmt.Println(err)
					continue
				}

			}

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
			fmt.Println("Enter transaction #: ")
			scanner.Scan()
			txnum, err := strconv.Atoi(scanner.Text())
			if err != nil {
				fmt.Println(err)
				break
			}
			if txnum >= len(block_chain.Blocks[block_num].Transactions) {
				fmt.Println("Invalid tx")
				break
			}
			fmt.Println("Decrementing output value by 1")
			block_chain.Blocks[block_num].Transactions[txnum].Outputs[0].Value -= 1
			fmt.Println("Reverifying block-chain: ")
			prev_block_length := len(block_chain.Blocks)
			blocks, err := block_chain.VerifyBlocks()
			fmt.Printf("Validating blocks: #%v discarded, %v\n", int32(prev_block_length)-blocks, err)

		case "printpeers":
			node.PrintConnectedPeers()
		case "printmerkletree":
			fmt.Println("Enter block hash: ")
			scanner.Scan()
			hash_hex := scanner.Text()
			bytes, err := hex.DecodeString(hash_hex)
			if err != nil {
				fmt.Println("Invalid hash")
				break
			}
			idx := block_chain.SearchBlockByHash([32]byte(bytes))
			if idx == nil {
				fmt.Println("Block does not exist")
				break
			}
			tree, err := blockchain.MakeTXMerkleTree(block_chain.Blocks[*idx].Transactions, uint32(*idx))
			if err != nil {
				log.Fatal(err)
			}
			tree.PrettyPrint()
		case "mempool":
			fmt.Println("Printing mempool")
			for _, tx := range block_chain.Mempool {
				fmt.Printf("%+v", tx)
			}
		case "exit":
			return
		}
	}
	block_chain.PrettyPrint()
}
