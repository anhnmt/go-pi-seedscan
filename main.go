package main

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Config struct {
	SeedPhrase string
}

var cfg Config

func init() {
	pflag.StringVar(&cfg.SeedPhrase, "s", "", "your seed phrase")
	pflag.Parse()
}

func main() {
	fmt.Printf("seed_phrase: %s [%d]\n", cfg.SeedPhrase, len(cfg.SeedPhrase))
}
