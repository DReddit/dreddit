package main

import (
  "testing"
  "time"
  "log"
  "node"
  "clerk"
  "fmt"
)

func TestBasic(t *testing.T) {
  fmt.Printf("Test: Basic setup with one miner and one user ...\n")
  const nservers = 1
  const nclients = 1
  const unreliable = false
  const tag = "basic"
  cfg = make_config(t, tag, nservers, unreliable)
  defer cfg.cleanup()

  ck := cfg.makeClient(cfg.All())

  

  fmt.Printf("  ... Passed\n")
}
