package tests

import (
	"clerk"
	"fmt"
	"node"
	"strconv"
	"testing"
	"time"
)

func cleanUp(drNodes []*node.DRNode) {
	for i := 0; i < len(drNodes); i++ {
		drNodes[i].Kill(nil, nil)
	}
}

func TestOnePost(t *testing.T) {
	fmt.Printf("Test: Basic setup with one miner, one user, one post ...\n")
	const nservers = 1
	const nclients = 1
	const unreliable = false
	const tag = "basic_one_post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+10000), empty)
		serverPorts[i] = strconv.Itoa(i + 10000)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+10100), serverPorts)
	}

	defer cleanUp(drNodes)

	ck := clients[0]
	ok := ck.Post("First Post!!!1")

	if !ok {
		t.Fatal("Attempted Post failed")
	}

	fmt.Printf("  ... Passed\n")
}

func TestMultiplePosts(t *testing.T) {
	fmt.Printf("Test: Basic setup with one miner, one user, ten posts ...\n")
	const nservers = 1
	const nclients = 1
	const unreliable = false
	const tag = "basic_multiple_posts"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+2000), empty)
		serverPorts[i] = strconv.Itoa(i + 2000)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), serverPorts)
	}

	defer cleanUp(drNodes)

	ck := clients[0]

	for i := 1; i <= 10; i++ {
		ok := ck.Post(fmt.Sprintf("Post #%d", i))
		if !ok {
			t.Fatal("Attempted Post failed")
		}
	}

	fmt.Printf("  ... Passed\n")
}

func TestGossip(t *testing.T) {
	fmt.Printf("Test: Setup with ten miners, one user, one post ...\n")
	const nservers = 10
	const nclients = 1
	const unreliable = false
	const tag = "basic_one_post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	drNodes[0] = node.StartDRNode(0, "13000", empty)
	serverPorts[0] = "13000"

	for i := 1; i < nservers; i++ {
		knownPorts := make([]string, 1)
		knownPorts[0] = strconv.Itoa(i + 12999)
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+13000), knownPorts)
		serverPorts[i] = strconv.Itoa(i + 13000)
	}

	defer cleanUp(drNodes)

	time.Sleep(time.Duration(3000) * time.Millisecond)

	for i := 0; i < nservers; i++ {
		dummyReply := node.DummyReply{}
		drNodes[i].GetPeerSize(nil, &dummyReply)
		size := dummyReply.RetVal
		if size != nservers-1 {
			t.Fatal("Did not learn group of peers quickly enough, miner %d only knows %d peers", i, size)
		}
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+3100), serverPorts)
	}

	ck := clients[0]
	ok := ck.Post("First Post!!!1")

	if !ok {
		t.Fatal("Attempted Post failed")
	}

	time.Sleep(time.Duration(2000) * time.Millisecond)

	fmt.Printf("  ... Passed\n")
}

func TestChainResolution(t *testing.T) {
	fmt.Printf("Test: Setup with five miners, five users, one post each ...\n")
	const nservers = 5
	const nclients = 5
	const unreliable = false
	const tag = "basic_one_post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	drNodes[0] = node.StartDRNode(0, "14000", empty)
	serverPorts[0] = "14000"

	for i := 1; i < nservers; i++ {
		knownPorts := make([]string, 1)
		knownPorts[0] = strconv.Itoa(i + 13999)
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+14000), knownPorts)
		serverPorts[i] = strconv.Itoa(i + 14000)
	}

	defer cleanUp(drNodes)

	time.Sleep(time.Duration(3000) * time.Millisecond)

	for i := 0; i < nservers; i++ {
		dummyReply := node.DummyReply{}
		drNodes[i].GetPeerSize(nil, &dummyReply)
		size := dummyReply.RetVal
		if size != nservers-1 {
			t.Fatal("Did not learn group of peers quickly enough, miner %d only knows %d peers", i, size)
		}
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+3100), serverPorts)
	}

	for i := 0; i < 5; i++ {
		go func(index int) {
			ck := clients[index]
			ok := ck.Post(strconv.Itoa(index) + " Post!!!1")

			if !ok {
				t.Fatal("Attempted Post failed")
			}
		}(i)
	}

	time.Sleep(time.Duration(10000) * time.Millisecond)

	fmt.Printf("  ... Passed\n")
}
