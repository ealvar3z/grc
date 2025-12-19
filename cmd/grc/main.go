package main

import (
	"fmt"
	"os"

	"grc/internal/parse"
)

func main() {
	_, err := parse.Parse(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
