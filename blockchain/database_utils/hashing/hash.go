package hash

//DO NOT EDIT

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
)

const (
	hashFormat  = "%s:%s" // salt:password
	hashRepeats = 100
)

func CreateSaltPasswordHash(salt, password string) string {
	sum := []byte(fmt.Sprintf(hashFormat, salt, password))
	return CreateHash(sum)
}

func CreateHash(sum []byte) string {
	var crutch [32]byte

	for i := 0; i < hashRepeats; i++ {
		crutch = sha256.Sum256(sum)
		sum = crutch[:]
	}

	return string(sum)
}

func HashUsername(username string) [20]byte {
	return sha1.Sum([]byte(username))
}
