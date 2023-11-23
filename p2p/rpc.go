// RPC for Blockchain. When a node connects to another node, an RPCResponder will be created on the node being connected to that handles events/requests
package p2p

import (
	"blockchain/blockchain"
	"errors"
	"log"
	"net"
)

type RPCResponder struct {
		node *Node
		conn net.Conn
}

func (rpc *RPCResponder) GetBlocks(args *struct{start int; end int}, reply *[]blockchain.Block) error {
	node := rpc.node
	rpc.node.lock.RLock()
	defer rpc.node.lock.RUnlock()
	if args.start > args.end {
		return errors.New("start > end")
	}
	if args.start >= len(node.block_chain.Blocks) {
		return errors.New("start > total number of blocks")
	}
	blocks := make([]blockchain.Block, 0, args.end - args.start)
	end := len(node.block_chain.Blocks)
	if args.end < end {
		end = end
	}
	for _, block := range node.block_chain.Blocks[args.start : args.end] {
		blocks = append(blocks, block)
	}
	*reply = blocks
	return nil
}


// This RPC call returns all the peers the RPC server is connected to
func (rpc *RPCResponder) GetPeers(args string, reply *[]string) error {
	node := rpc.node
	node.lock.RLock()
	defer node.lock.RUnlock()
	log.Println("RPCResponder.PrintPeers called")
	*reply = make([]string, 0)
	for addr, _ := range node.peers {
		*reply = append(*reply, addr)
	}
	return nil
}
