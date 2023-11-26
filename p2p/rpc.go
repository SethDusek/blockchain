// RPC for Blockchain. When a node connects to another node, an RPCResponder will be created on the node being connected to that handles events/requests
package p2p

import (
	"blockchain/blockchain"
	"errors"
	"log"
	"net"
	"strconv"
)

type RPCResponder struct {
	Node *Node
	Conn net.Conn
}

func NewRPCResponder(node *Node, conn net.Conn) RPCResponder {
	return RPCResponder{node, conn}
}

// Request blocks[start..end]. If end == start == -1 then return all blocks
func (rpc *RPCResponder) GetBlocks(args *struct {
	Start int
	End   int
	HeadersOnly bool
}, reply *[]blockchain.Block) error {
	node := rpc.Node
	rpc.Node.lock.RLock()
	defer rpc.Node.lock.RUnlock()

	if args.Start > args.End {
		return errors.New("start > end")
	}
	if args.Start >= len(node.block_chain.Blocks) {
		return errors.New("start > total number of blocks")
	}
	start, end := 0, 0
	if args.Start == args.End && args.End == -1 {
		start = 0
		end = len(node.block_chain.Blocks)
	} else if args.End <= len(node.block_chain.Blocks) {
		end = args.End
	} else {
		return errors.New("end > total Number of blocks")
	}
	log.Println("Start, end:", start,end)
	blocks := make([]blockchain.Block, 0, args.End-args.Start)

	for _, block := range node.block_chain.Blocks[start:end] {
		blocks = append(blocks, block)
		if args.HeadersOnly {
			blocks[len(blocks)-1].Transactions = make([]blockchain.Transaction, 0)
		}
	}
	*reply = blocks
	return nil
}

// Return count blocks after block_hash
func (rpc *RPCResponder) GetBlocksByHash(args struct {BlockHash [32]byte; Count uint}, reply *[]blockchain.Block) error {
	node := rpc.Node
	rpc.Node.lock.RLock()
	defer rpc.Node.lock.RUnlock()

	if idx := node.block_chain.SearchBlockByHash(args.BlockHash); idx == nil {
		return errors.New("Block Not Found")
	} else {
		*reply = make([]blockchain.Block, 0, args.Count)
		end := *idx + int(args.Count)
		if end >= len(node.block_chain.Blocks) {
			end = len(node.block_chain.Blocks)
		}
		for i := *idx; i < end; i++ {
			*reply = append(*reply, node.block_chain.Blocks[i])
		}
		return nil
	}
}


// This RPC call returns all the peers the RPC server is connected to
func (rpc *RPCResponder) GetPeers(args string, reply *[]string) error {
	node := rpc.Node
	node.lock.RLock()
	defer node.lock.RUnlock()
	log.Println("RPCResponder.GetPeers called")
	*reply = make([]string, 0)
	for addr := range node.peers {
		*reply = append(*reply, addr)
	}
	return nil
}

// Signals to the RPC server that the client is accepting inbound connections on their node on port *args*
func (rpc *RPCResponder) Connect(port uint16, reply *string) error {
	log.Println("RPCResponder.Connect called")
	var conn_addr string
	switch addr := rpc.Conn.RemoteAddr().(type) {
	case *net.UDPAddr:
		conn_addr = addr.IP.String()
	case *net.TCPAddr:
		conn_addr = addr.IP.String()
	}
	conn_addr += ":" + strconv.Itoa(int(port))
	log.Println("Attempting connection to", conn_addr)
	go rpc.Node.Connect(conn_addr)
	*reply = ""
	return nil
}

func (rpc *RPCResponder) NewBlock(block blockchain.Block, reply *string) error {
	log.Println("RPCResponder.NewBlock called")
	log.Printf("%+x", block)
	if err := rpc.Node.NewBlock(block, rpc.Conn.RemoteAddr().String()); err != nil {
		log.Println("RPCResponder.NewBlock error", err)
	}
	return nil
}

func (rpc *RPCResponder) NewTransaction(tx blockchain.Transaction, reply *string) error {
	log.Println("RPCResponder.NewTransaction called")
	log.Printf("%+x\n", tx)
	if err := rpc.Node.NewTransaction(tx, rpc.Conn.RemoteAddr().String()); err != nil {
		log.Println("RPCResponder.NewTransaction error", err)
	}
	return nil
}

// func (rpc *RPCResponder) ConvertToClient(args string, reply string) error {

// }
