package node

import (
	"golang.org/x/crypto/scrypt"
)

//
// Uses scrypt to hash data
func hash(data []byte) []byte {
	dk, _ := scrypt.Key(data, data, 1024, 1, 1, 32)
	return dk
}
