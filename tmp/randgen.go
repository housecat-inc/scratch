package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func main() {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}

	fmt.Println(n)
}
