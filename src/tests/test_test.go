package tests

import (
	"fmt"
	"testing"
)

func TestOnePost(t *testing.T) {
	fmt.Printf("Test: Basic setup with one miner, one user, one post ...\n")
	const nservers = 1
	const nclients = 1
	const unreliable = false
	const tag = "basic_one_post"
	cfg := make_config(t, tag, nservers, unreliable)
	defer cfg.cleanup()

	ck := cfg.makeClient(cfg.All())
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
	cfg := make_config(t, tag, nservers, unreliable)
	defer cfg.cleanup()

	ck := cfg.makeClient(cfg.All())

	for i := 1; i <= 10; i++ {
		ok := ck.Post(fmt.Sprintf("Post #%d", i))
		if !ok {
			t.Fatal("Attempted Post failed")
		}
	}

	fmt.Printf("  ... Passed\n")
}
