package p2p

import (
	"blockchain/blockchain"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sort"
	"sync"
	"time"
)

// TODO: add mutexes for both node access itself and also block_chain
type Node struct {
	lock    sync.RWMutex
	address string
	// The port the node is listening on
	port        uint16
	block_chain *blockchain.BlockChain
	peers       map[string]*rpc.Client
	// This is a list of listeners/goroutines that should be notified when something in the blockchain changes (new block, new transaction, etc)
	changed_listeners []chan bool
}

func NewNode(server net.Listener, block_chain *blockchain.BlockChain) *Node {
	var port uint16
	switch addr := server.Addr().(type) {
	case *net.UDPAddr:
		port = addr.AddrPort().Port()
	case *net.TCPAddr:
		port = addr.AddrPort().Port()
	}
	node := Node{sync.RWMutex{}, server.Addr().String(), port, block_chain, make(map[string]*rpc.Client, 0), make([]chan bool, 0)}
	log.Printf("Listening on %v\n", server.Addr())
	// Start the RPC responder service
	go func(server net.Listener) {
		for {
			conn, err := server.Accept()
			if err != nil {
				log.Println("Error accepting connection on server", err)
			}
			log.Println("New connection from", conn.RemoteAddr().String())
			responder := NewRPCResponder(&node, conn)
			server := rpc.NewServer()
			if err := server.Register(&responder); err != nil {
				log.Println("Error registering RPC methods", err)
				conn.Close()
			}
			go server.ServeConn(conn)
		}
	}(server)
	return &node
}

// Create a new channel that will be signaled whenever the blockchain changes
func (node *Node) NewListener() chan bool {
	new_channel := make(chan bool, 10)
	node.lock.Lock()
	defer node.lock.Unlock()
	node.changed_listeners = append(node.changed_listeners, new_channel)
	return new_channel
}

// Signal that something in the blockchain has changed to all listeners
func (node *Node) signal_changed() {
	node.lock.Lock()
	defer node.lock.Unlock()
	for _, listener := range node.changed_listeners {
		select {
		case listener <- true:
			continue
		case <-time.After(200 * time.Millisecond):
			log.Println("Timeout reached on signal")
		}
	}
}

// Starts mining and creates two channels. One will be used by the mining goroutine to signal new blocks, and another will be used by the node when a new block is found and to tell the miner to start building on new longest chain
func (node *Node) StartMiner() {
	log.Println("Starting miner")
	// The miner thread will send a block here once it's mined
	rcv_channel := make(chan blockchain.BlockHeader, 1)
	// When a new block is found by some other node, or a new transaction is created, we will send a new block candidate to the miner thread through this channel
	send_channel := make(chan blockchain.BlockHeader, 1)
	// Used to signal when something in the blockchain has changed which indicates we should build a new block candidate
	listener := node.NewListener()
	go func(send_channel chan blockchain.BlockHeader, rcv_channel chan blockchain.BlockHeader) {
		node.lock.RLock()
		candidate, err := node.block_chain.NewBlockCandidate()
		node.lock.RUnlock()
		if err != nil {
			log.Fatal(err)
		}
		go blockchain.MineBlock(candidate.Header, send_channel, rcv_channel)
		for {
			select {
			// Something in block-chain has changed, create a new block candidate and send it to miner
			case <-listener:
				log.Println("Building new block candidate at block height", len(node.block_chain.Blocks))
				node.lock.RLock()
				candidate, err = node.block_chain.NewBlockCandidate()
				node.lock.RUnlock()
				log.Println("Built new block candidate at block height", len(node.block_chain.Blocks))
				if err != nil {
					log.Fatal(err)
				}
				go func() { send_channel <- candidate.Header }()
				log.Println("Sent block candidate to miner")
			case mined_header := <-rcv_channel:
				log.Println("Miner has found new block! Broadcasting")
				candidate.Header = mined_header
				node.NewBlock(*candidate, "")
			}

		}
	}(send_channel, rcv_channel)

}

func (node *Node) Connect(address string) error {
	_, already_connected := node.peers[address]
	if !already_connected {
		log.Println("Node connecting to ", address)
		client, err := rpc.Dial("tcp", address)
		if err != nil {
			log.Println(err)
			return err
		}
		node.lock.Lock()
		defer node.lock.Unlock()
		node.peers[address] = client
		var reply string
		if err := client.Call("RPCResponder.Connect", node.port, &reply); err != nil {
			log.Println("Error calling RPCResponder.Connect", err)
		}
	} else {
		log.Println("Already connected to", address)
	}

	return nil
}

// This is *not* an RPC call. It's simply used for debugging. It prints all of the peers of nodes we're connected to
func (node *Node) PrintConnectedPeers() {
	node.lock.RLock()
	defer node.lock.RUnlock()
	for addr, peer := range node.peers {
		log.Println("Requesting peers of", addr)
		reply := make([]string, 0)
		err := peer.Call("RPCResponder.GetPeers", "", &reply)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, v := range reply {
			fmt.Println("reply", v)
		}
	}
}

// Synchronizes node with longest chain on network.
func (node *Node) Sync() {
	log.Println("Node.Sync called")
	// Request headers

	// Channel used to receive list of block headers
	type channel_value struct {
		peer_addr string
		blocks    []blockchain.Block
	}
	channel := make(chan channel_value, len(node.peers))

	// We first request all headers from peers asynchronously
	i := 0
	for addr, peer := range node.peers {
		args := struct {
			Start int
			End   int
			bool
		}{-1, -1, true}
		go func(channel chan<- channel_value) {
			headers := make([]blockchain.Block, 0)
			if err := peer.Call("RPCResponder.GetBlocks", args, &headers); err != nil {
				log.Println("Error calling RPCResponder.GetBlocks on peer", addr, err)
				return
			}
			channel <- channel_value{addr, headers}
		}(channel)
		i++
	}
	// Sorted by max length
	var peer_headers [][]blockchain.Block
	// Receive list of block headers from all peers, and select the longest list of headers that passes verification
outer:
	for {
		select {
		case headers := <-channel:
			all := true
			for i := range headers.blocks {
				if !blockchain.VerifyBlockHeader(headers.blocks, uint32(i)) {
					log.Println("Verification of headers from ", headers.peer_addr, "failed")
					all = false
					break
				}
			}
			if !all {
				continue
			}
			peer_headers = append(peer_headers, headers.blocks)
		case <-time.After(time.Second * 20):
			fmt.Println("Timeout reached")
			break outer
		default:
			if len(peer_headers) == len(node.peers) {
				break outer
			}
		}
	}
	sort.Slice(peer_headers, func(i, j int) bool {
		return len(peer_headers[i]) > len(peer_headers[j])
	})
	if len(peer_headers) == 0 {
		log.Println("All peer headers are invalid or of 0 length. Node is synchronized")
		return
	}
	log.Println("Max block headers received", len(peer_headers[0]))

	if len(peer_headers[0]) <= len(node.block_chain.Blocks) {
		log.Println("Node is already synchronized, network #headers:", len(peer_headers[0]), "we have:", len(node.block_chain.Blocks))
	}
	node.lock.RLock()
	// Request full blocks from all peers
	blocks := make([]blockchain.Block, 0)
	for _, header := range peer_headers[0] {
		log.Printf("Requesting full block %x\n", header.Header.BlockHash())
		args := struct {
			BlockHash [32]byte
			Count     uint
		}{[32]byte(header.Header.BlockHash()), 1}
		channel := make(chan blockchain.Block)
		// This isn't as efficient as it could be, we send a request to all peers for a certain block and get the first one that responds
		for addr, peer := range node.peers {
			go func(peer *rpc.Client, addr string, channel chan<- blockchain.Block) {
				reply := make([]blockchain.Block, 1)
				if err := peer.Call("RPCResponder.GetBlocksByHash", args, &reply); err != nil {
					log.Println("Error getting block from peer", addr, err)
				}
				if len(reply) != 1 {
					log.Println("Peer", addr, "did not send enough blocks")
				} else {
					channel <- reply[0]
				}
			}(peer, addr, channel)
		}
		block := <-channel
		blocks = append(blocks, block)
	}
	node.lock.RUnlock()

	node.lock.Lock()
	log.Println("Sync completed, attempting to build longest chain")
	log.Println("AttemptOrphan result", node.block_chain.AttemptOrphan(blocks))
	node.lock.Unlock()
}

// Adds a new block to the blockchain and floods it to all peers except src
func (node *Node) NewBlock(block blockchain.Block, src string) error {
	node.lock.Lock()

	var err error
	if err = node.block_chain.AddBlock(block); err != nil {
		log.Println("Error adding new block to chain", err)
	}
	// If the block we've been given does not match our longest chain, request a bunch of blocks previous to this block
	if block.Header.PrevHash != [32]byte(node.block_chain.Blocks[len(node.block_chain.Blocks)-1].Header.BlockHash()) {
		log.Println("Maybe found new longest chain, attempting to synchronize")
		maybe_longer_chain := make([]blockchain.Block, 0)
		maybe_longer_chain = append(maybe_longer_chain, block)
	outer:
		for {
			prev_hash := maybe_longer_chain[len(maybe_longer_chain)-1].Header.PrevHash
			idx := node.block_chain.SearchBlockByHash(prev_hash)
			if idx != nil {
				break
			}
			channel := make(chan blockchain.Block, len(node.peers))
			// Request block who's prev hash we don't know from all peers
			for _, peer := range node.peers {
				go func(peer *rpc.Client, channel chan<- blockchain.Block) {
					reply := make([]blockchain.Block, 0)
					if err := peer.Call("RPCResponder.GetBlocksByHash", struct {
						BlockHash [32]byte
						Count     uint
					}{prev_hash, 1}, &reply); err != nil {
						log.Println("Error calling RPCResponder.GetBlocksByHash", err)
					}
					if len(reply) != 0 {
						channel <- reply[0]
					}
				}(peer, channel)
			}
			select {
			case prev_block := <-channel:
				maybe_longer_chain = append(maybe_longer_chain, prev_block)
				prev_hash = prev_block.Header.PrevHash
			case <-time.After(time.Second * 10):
				log.Println("Timeout reached when requesting prev blocks")
				break outer
			}
		}
		// Reverse the chain, since we're requesting blocks from the latest block all the way back
		for i, j := 0, len(maybe_longer_chain)-1; i < j; i, j = i+1, j-1 {
			maybe_longer_chain[i], maybe_longer_chain[j] = maybe_longer_chain[j], maybe_longer_chain[i]
		}
		prev_length := len(node.block_chain.Blocks)
		if node.block_chain.AttemptOrphan(maybe_longer_chain) {
			log.Printf("New longest chain has been found! Previous length: %v, new length: %v\n", prev_length, len(node.block_chain.Blocks))
			err = nil
		} else {
			log.Printf("We have the longest chain\n")
		}

	}

	node.lock.Unlock()
	if err != nil {
		return err
	}
	node.lock.RLock()
	for addr, peer := range node.peers {
		if addr == src {
			continue
		}
		var reply string
		call := peer.Go("RPCResponder.NewBlock", block, &reply, nil)
		go func(call *rpc.Call) {
			<-call.Done
			if call.Error != nil {
				log.Println("Error calling RPCResponder.NewBlock", call.Error)
			}
		}(call)
	}
	node.lock.RUnlock()
	node.signal_changed()
	return nil
}

// Adds a new transaction to the mempool and floods it to all peers except src
func (node *Node) NewTransaction(tx blockchain.Transaction, src string) error {
	node.lock.Lock()
	log.Println("Node.NewTransaction called")

	if err := node.block_chain.AddTXToMempool(tx); err != nil {
		log.Println("Error adding new transaction to mempool", err)
		node.lock.Unlock()
		return err
	}
	for _, peer := range node.peers {
		var reply string
		call := peer.Go("RPCResponder.NewTransaction", tx, &reply, nil)
		go func(call *rpc.Call) {
			<-call.Done
			if call.Error != nil {
				log.Println("Error calling RPCResponder.Transaction", call.Error)
			}
		}(call)
	}
	node.lock.Unlock()
	node.signal_changed()
	return nil
}
