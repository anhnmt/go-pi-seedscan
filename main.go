package main

import (
	"github.com/spf13/pflag"
)

type Config struct {
	SeedPhrase     string
	DerivationPath string
	MaxWordMissing int
	BatchSize      int
}

var cfg Config

func main() {
	pflag.StringVar(&cfg.SeedPhrase, "s", "", "your seed phrase")
	pflag.StringVar(&cfg.DerivationPath, "d", "m/44'/314159'/0'", "derivation path")
	pflag.IntVar(&cfg.MaxWordMissing, "m", 10, "max word missing")
	pflag.IntVar(&cfg.BatchSize, "b", 10, "batch size")
	pflag.Parse()

	recoverSeedPhrase(cfg)
}
