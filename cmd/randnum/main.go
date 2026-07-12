package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
)

func main() {
	count := flag.Int("count", 1, "how many numbers to generate")
	max := flag.Int("max", 100, "maximum generated number")
	min := flag.Int("min", 1, "minimum generated number")
	flag.Parse()

	if *count < 1 {
		fmt.Fprintln(os.Stderr, "count must be at least 1")
		os.Exit(1)
	}

	if *min > *max {
		fmt.Fprintln(os.Stderr, "min must be less than or equal to max")
		os.Exit(1)
	}

	for range *count {
		fmt.Println(rand.IntN(*max-*min+1) + *min)
	}
}
