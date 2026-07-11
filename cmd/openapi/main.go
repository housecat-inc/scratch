package main

import (
	"fmt"
	"os"

	"github.com/housecat-inc/scratch/pkg/api"
)

func main() {
	api.Spec("docs/openapi.json")
	fmt.Fprintln(os.Stderr, "wrote docs/openapi.json")
}
