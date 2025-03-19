package main

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

type Config struct {
	SeedPhrase     string
	DerivationPath string
	MaxWordMissing int
	BatchSize      int
	Debug          bool
}

var cfg Config

func main() {
	pflag.StringVar(&cfg.SeedPhrase, "s", "", "your seed phrase")
	pflag.StringVar(&cfg.DerivationPath, "p", "m/44'/314159'/0'", "derivation path")
	pflag.IntVar(&cfg.MaxWordMissing, "m", 5, "max word missing")
	pflag.IntVar(&cfg.BatchSize, "b", 10, "batch size")
	pflag.BoolVar(&cfg.Debug, "d", true, "debug mod")
	pflag.Parse()

	// init logger
	initLogger(cfg.Debug)

	if cfg.SeedPhrase == "" {
		log.Fatal().Msg("Seed phrase is empty")
	}

	// recover seed phrase
	recoverSeedPhrase(cfg)
}
