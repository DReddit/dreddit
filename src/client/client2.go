package main

import (
	"bufio"
	"bytes"
	"clerk"
	"fmt"
	"net/rpc"
	"node"
	"os"
	"strconv"
	"strings"
	"util"
)

type TxClient struct {
	tx   node.Transaction // the transaction itself
	hash string           // its hash

	// Only for posts or comments
	score   int    // number of upvotes
	address string // the address of its author
}

type Client struct {
	clerkPort string      // address of clerk
	nodePort  string      // address of drnode
	server    *rpc.Client // dreddit node the client knows of

	hashMap map[string]*TxClient // map of tx abbreviations to tx hashes
}

func MakeClient(clerkPort string, nodePort string) *Client {
	cl := new(Client)
	cl.clerkPort = clerkPort
	cl.nodePort = nodePort

	cl.hashMap = make(map[string]*TxClient)

	client, err := rpc.DialHTTPPath("tcp", "localhost:"+nodePort, "/dreddit"+nodePort)
	if err == nil {
		cl.server = client
	}

	return cl
}

//
// Periodically queries the current state of the blockchain
// and then renders a "front page" as ASCII text
func (cl *Client) QueryBlockchain() {
	args := node.BlockArgs{}
	reply := node.BlockReply{}
	cl.server.Call("DRNode.GetBlockchain", &args, &reply)
	blockchain := reply.Log
	fmt.Printf("               _____ _          ___ _         _      _         _    \n")
	fmt.Printf("              |_   _| |_  ___  | _ ) |___  __| |____| |_  __ _(_)_ _ \n")
	fmt.Printf("                | | | ' \\/ -_) | _ \\ / _ \\/ _| / / _| ' \\/ _` | | ' \\ \n")
	fmt.Printf("                |_| |_||_\\___| |___/_\\___/\\__|_\\_\\__|_||_\\__,_|_|_||_|\n")
	fmt.Printf("                                                                     \n")
	for _, block := range blockchain {
		fmt.Printf("%s\n", block.ToString())
	}
	cl.renderFrontPage(blockchain)
	fmt.Printf("\n")
}

func (cl *Client) renderFrontPage(blockchain []node.Block) {
	fmt.Printf("\n\n")
	fmt.Printf("                __  __         ___              __                    \n")
	fmt.Printf("               / /_/ /  ___   / _/______  ___  / /____  ___ ____ ____ \n")
	fmt.Printf("              / __/ _ \\/ -_) / _/ __/ _ \\/ _ \\/ __/ _ \\/ _ `/ _ `/ -_)\n")
	fmt.Printf("              \\__/_//_/\\__/ /_//_/  \\___/_//_/\\__/ .__/\\_,_/\\_, /\\__/ \n")
	fmt.Printf("                                                /_/        /___/      \n\n\n")
	txs := make([]node.Transaction, 0)

  // Grab all the transactions from all the blocks
	for _, block := range blockchain {
		txs = append(txs, block.Transactions...)
	}

  // Create TxClient structs for all of them
  for _, tx := range txs {
    hash := tx.Hash()
    cl.hashMap[fmt.Sprintf("%x\n", hash[0:6])] = &TxClient{tx, string(hash), 0, ""}

    if tx.Type == node.POST || tx.Type == node.COMMENT {
      address := string(tx.TxOuts[0].PubKeyHash)
      cl.hashMap[fmt.Sprintf("%x\n", hash[0:6])].address = address
    }
  }

  // Count upvotes
  for _, tx := range txs {
    if tx.Type == node.UPVOTE {
      cl.hashMap[fmt.Sprintf("%x\n", tx.Parent[0:6])].score++
    }
  }

	for _, tx := range txs {
		if tx.Type == node.POST {
			cl.renderContent(tx, txs, 0)
			fmt.Printf("\n")
		}
	}
}

func (cl *Client) renderContent(tx node.Transaction, txs []node.Transaction, indent int) {
  hash := tx.Hash()
	fmt.Printf(strings.Repeat("      ", indent) + "                    /-----\\--------------------------------------\n")
	fmt.Printf(strings.Repeat("      ", indent) + "                    |  %d  |   %s", cl.hashMap[fmt.Sprintf("%x\n", hash[0:6])].score, tx.Content)
	fmt.Printf(strings.Repeat("      ", indent) + "                    \\-----/--------------------------------------\n")
	fmt.Printf(strings.Repeat("      ", indent) + "                                                \\  %x\n", hash[0:6])
	fmt.Printf(strings.Repeat("      ", indent) + "                                                 \\---------------\n")

	for _, txd := range txs {
		if txd.Type == node.COMMENT && bytes.Equal(txd.Parent, hash) {
			cl.renderContent(txd, txs, indent+1)
		}
	}
}

func main() {
	fmt.Printf("                               _           _   _ _ _   \n")
	fmt.Printf("                             _| |___ ___ _| |_| |_| |_ \n")
	fmt.Printf("                            | . |  _| -_| . | . | |  _|\n")
	fmt.Printf("                            |___|_| |___|___|___|_|_|  \n")
	fmt.Printf("                                                       \n")

	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1

	const nservers = 1
	const nclients = 1

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	nonempty := make([]string, 1)
	nonempty[0] = strconv.Itoa(10000)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()

	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+20000), pkPairs[i].PubHash, nonempty)
		serverPorts[i] = strconv.Itoa(i + 20000)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+20200), pkPairs[nservers+i].Priv, serverPorts)
	}

	cl := MakeClient(strconv.Itoa(20200), strconv.Itoa(20000))

	ck := clients[0]

	reader := bufio.NewReader(os.Stdin)
	quit := false
	for {
		fmt.Printf("> ")
		text, _ := reader.ReadString('\n')
		switch text {
		case "help\n":
			fmt.Printf("Available commands:\n")
			fmt.Printf("  * help\n")
			fmt.Printf("  * post\n")
			fmt.Printf("  * comment\n")
			fmt.Printf("  * upvote\n")
			fmt.Printf("  * view\n")
			fmt.Printf("  * quit\n")
		case "view\n":
			cl.QueryBlockchain()
		case "post\n":
			fmt.Printf("Content: ")
			text, _ := reader.ReadString('\n')
			ok := ck.Post(text)
			if ok {
				fmt.Printf("Success!\n")
			} else {
				fmt.Printf("Failed :(\n")
			}
		case "comment\n":
			fmt.Printf("Parent: ")
			text, _ := reader.ReadString('\n')
			txc, ok := cl.hashMap[text]
			if !ok {
				fmt.Printf("Couldn't find parent\n")
			} else {
				fmt.Printf("Content: ")
				text, _ := reader.ReadString('\n')
				ok = ck.Comment(txc.hash, text)
				if ok {
					fmt.Printf("Success!\n")
				} else {
					fmt.Printf("Failed :(\n")
				}
			}
		case "upvote\n":
			fmt.Printf("Parent: ")

			text, _ := reader.ReadString('\n')
			txc, ok := cl.hashMap[text]
			if !ok {
				fmt.Printf("Couldn't find parent\n")
			} else {
				ok = ck.Upvote(txc.hash, txc.address)
				if ok {
					fmt.Printf("Success!\n")
				} else {
					fmt.Printf("Failed :(\n")
				}
			}
		case "quit\n":
			quit = true
		default:
			fmt.Printf("not a recognized command\n")
		}
		if quit {
			break
		}
	}
}
