package node

import (
  "golang.org/x/crypto/scrypt"
)

//
// Uses scrypt to hash data
func hash(data []byte) []byte {
  // TODO check that data is 80 bytes
  // and do something if it isn't
  dk, _ := scrypt.Key(data, data, 1024, 1, 1, 32)
  return dk
}
