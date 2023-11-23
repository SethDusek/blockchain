package p2p

import (
	"blockchain/blockchain"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
)

// TODO: add mutexes for both node access itself and also block_chain
type Node struct {
	lock sync.RWMutex
	address string
	block_chain *blockchain.BlockChain
	peers map[string]*rpc.Client
}

func (node *Node) Connect(address string) error {
	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		return err
	}
	node.peers[address] = client
	var reply string
	// We don't care about errors here, just telling the node we're connecting to that they can also connect to us
	// TODO: this probably won't actually work, because listener.Addr() gives 0.0.0.0 or something similar whereas we need our actual IP
	client.Call("Node.ConnectTo", node.address, &reply)
	return nil
}

// This is *not* an RPC call. It's simply used for debugging. It prints all of the peers of nodes we're connected to
func (node *Node) PrintConnectedPeers() {
	for addr, peer := range node.peers {
		log.Println("Requesting peers of ", addr)
		reply := make([]string, 0)
		err := peer.Call("Node.GetPeers", "", &reply)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, v := range reply {
			fmt.Println("reply", v)
		}
	}
}

func NewNode(server net.Listener, block_chain *blockchain.BlockChain) *Node {
	node := Node{sync.RWMutex{}, server.Addr().String(), block_chain, make(map[string]*rpc.Client, 0)}
	rpc.Register(&node)
	rpc.HandleHTTP()
	log.Printf("Listening on %v\n", server.Addr())
	go http.Serve(server, nil)
	return &node
}
