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
}

func NewNode(server net.Listener, block_chain *blockchain.BlockChain) *Node {
	var port uint16
	switch addr := server.Addr().(type) {
	case *net.UDPAddr:
		port = addr.AddrPort().Port()
	case *net.TCPAddr:
		port = addr.AddrPort().Port()
	}
	node := Node{sync.RWMutex{}, server.Addr().String(), port, block_chain, make(map[string]*rpc.Client, 0)}
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
				node.lock.RUnlock()
				return
			}
			channel <- channel_value{addr, headers}
		}(channel)
		i++
	}
	// Sorted by max length
	var peer_headers [][]blockchain.Block
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

	node.lock.RLock()
	if len(peer_headers[0]) <= len(node.block_chain.Blocks) {
		log.Println("Node is already synchronized, network #headers:", len(peer_headers[0]), "we have:", len(node.block_chain.Blocks))
	}
	node.lock.RUnlock()

	node.lock.RLock()
	// Request full blocks from all peers
	blocks := make([]blockchain.Block, 0)
	for _, header := range peer_headers[0] {
		log.Printf("Requesting full block %x\n", header.Header.BlockHash())
		args := struct{BlockHash [32]byte; Count uint}{[32]byte(header.Header.BlockHash()), 1}
		channel := make(chan blockchain.Block)
		// This isn't as efficient as it could be, we send a request to all peers for a certain block and get the first one that responds
		for addr, peer := range node.peers {
			go func(peer *rpc.Client, addr string, channel chan <- blockchain.Block) {
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
		block := <- channel
		blocks = append(blocks, block)
	}
	node.lock.RUnlock()
	log.Println("Sync completed, attempting to build longest chain")
	node.lock.Lock()
	log.Println("AttemptOrphan result", node.block_chain.AttemptOrphan(blocks))
}
