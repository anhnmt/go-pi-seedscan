package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/exp/crypto/derivation"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/tyler-smith/go-bip39"
)

const (
	DefaultMainNetURL = "https://api.mainnet.minepi.com"
	DefaultTestNetURL = "https://api.testnet.minepi.com"
)

const (
	DerivationPath = "m/44'/314159'/0'"
)

type Config struct {
	SeedPhrase     string
	DerivationPath string
	MaxWordMissing int
	BatchSize      int
	Testnet        bool
	Debug          bool
}

var cfg Config

func main() {
	pflag.StringVarP(&cfg.SeedPhrase, "seed", "s", "", "Your seed phrase")
	pflag.IntVarP(&cfg.MaxWordMissing, "max_word", "m", 5, "Max missing words allowed")
	pflag.IntVarP(&cfg.BatchSize, "batch", "b", 10, "Batch size for processing")
	pflag.BoolVarP(&cfg.Testnet, "testnet", "t", false, "Use Testnet instead of Mainnet")
	pflag.BoolVarP(&cfg.Debug, "debug", "d", false, "Enable debug mode")
	pflag.Parse()

	initLogger(cfg.Debug)

	if err := validateConfig(cfg); err != nil {
		log.Fatal().Msgf("Invalid config: %v", err)
	}

	// Start seed phrase recovery process
	RecoverSeedPhrase(cfg)
}

// validateConfig checks if the required parameters are provided and valid.
func validateConfig(cfg Config) error {
	if cfg.SeedPhrase == "" {
		return fmt.Errorf("seed phrase cannot be empty")
	}
	if cfg.MaxWordMissing < 0 || cfg.MaxWordMissing > 24 {
		return fmt.Errorf("max_word must be between 0 and 24")
	}
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0")
	}
	return nil
}

// initLogger initializes the global logger with the specified log level and formatting.
func initLogger(debug bool) {
	// Set logging level based on debug mode
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure time format and custom JSON marshaller
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.InterfaceMarshalFunc = sonic.Marshal

	// Customize caller output to show only the filename and line number
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	// Configure the logger with console output, timestamp, and caller info
	log.Logger = zerolog.
		New(&zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}).
		With().
		Timestamp().
		Caller().
		Logger()
}

// GetHorizonURL selects the appropriate Horizon API URL based on whether Testnet or Mainnet is used.
func GetHorizonURL(testnet bool) string {
	if testnet {
		return DefaultTestNetURL
	}
	return DefaultMainNetURL
}

// GetPiWallet generates a Pi Network wallet address from a Seed Phrase with a flexible derivation path.
func GetPiWallet(seedPhrase string) (string, error) {
	// Generate seed from mnemonic
	seed := bip39.NewSeed(seedPhrase, "")

	// Derive key using the specified derivation path
	derivedKey, err := derivation.DeriveForPath(DerivationPath, seed)
	if err != nil {
		return "", errors.Wrap(err, "failed to derive key")
	}

	// Extract the first 32 bytes as the Ed25519 seed
	var ed25519Seed [32]byte
	copy(ed25519Seed[:], derivedKey.Key[:32]) // Convert to [32]byte

	// Generate Stellar Keypair (for Pi Network)
	kp, err := keypair.FromRawSeed(ed25519Seed)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate Stellar keypair")
	}

	return kp.Address(), nil
}

// GetAccountBalance retrieves the balance of a Stellar wallet address.
func GetAccountBalance(address string, horizonURL string) (*horizon.Account, error) {
	client := horizonclient.Client{HorizonURL: horizonURL}
	accountRequest := horizonclient.AccountRequest{AccountID: address}

	// Fetch account details from Horizon API
	account, err := client.AccountDetail(accountRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account details: %w", err)
	}

	return &account, nil
}

// recoverMissingWords recursively tests all possible word combinations to restore a valid Seed Phrase.
func recoverMissingWords(words []string, missingIndexes []int, index int, wg *sync.WaitGroup, results chan<- string) {
	// Base case: when all missing words are filled, validate the Seed Phrase
	if index >= len(missingIndexes) {
		testPhrase := strings.Join(words, " ")
		if bip39.IsMnemonicValid(testPhrase) {
			results <- testPhrase
		}
		return
	}

	// Iterate through all possible words in the BIP-39 word list
	for _, word := range bip39.GetWordList() {
		words[missingIndexes[index]] = word
		recoverMissingWords(words, missingIndexes, index+1, wg, results)
	}
}

// RecoverSeedPhrase attempts to restore a valid Seed Phrase by filling in missing words.
func RecoverSeedPhrase(cfg Config) {
	words := strings.Split(cfg.SeedPhrase, " ")

	// If the Seed Phrase is complete (24 words) and contains no missing words, validate it immediately
	if len(words) == 24 && !strings.Contains(cfg.SeedPhrase, "?") {
		validateSeedPhrase(cfg.SeedPhrase)
		return
	}

	// Identify missing word positions
	missingIndexes := findMissingIndexes(words)

	// Validate the number of missing words
	if !validateMissingWords(len(missingIndexes), cfg.MaxWordMissing) {
		return
	}

	// Channel to receive valid Seed Phrases from goroutines
	results := make(chan string, cfg.BatchSize)
	var wg sync.WaitGroup
	wg.Add(1)

	// Start recursive recovery process
	go func() {
		defer wg.Done()
		recoverMissingWords(words, missingIndexes, 0, &wg, results)
	}()

	// Close the results channel once processing is complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process the recovered Seed Phrases
	processRecoveredPhrases(results, GetHorizonURL(cfg.Testnet))
}

// validateSeedPhrase checks if the given Seed Phrase is valid.
func validateSeedPhrase(seedPhrase string) {
	if bip39.IsMnemonicValid(seedPhrase) {
		log.Info().Msg("✅ Valid Seed Phrase!")
	} else {
		log.Error().Msg("❌ Invalid Seed Phrase!")
	}
}

// findMissingIndexes returns the indexes of missing words in the Seed Phrase.
func findMissingIndexes(words []string) []int {
	var missingIndexes []int
	for i, word := range words {
		if word == "?" {
			missingIndexes = append(missingIndexes, i)
		}
	}
	return missingIndexes
}

// validateMissingWords checks if the number of missing words is within the allowed limit.
func validateMissingWords(missingCount, maxAllowed int) bool {
	switch {
	case missingCount == 0:
		log.Info().Msg("No missing words detected.")
		return false
	case missingCount > maxAllowed:
		log.Error().Msgf("🚨 Only up to %d missing words can be recovered.", maxAllowed)
		return false
	}
	return true
}

// processRecoveredPhrases handles the recovered Seed Phrases and checks their account balance.
func processRecoveredPhrases(results <-chan string, horizonURL string) {
	found := false

	for phrase := range results {
		publicAddress, err := GetPiWallet(phrase)
		if err != nil {
			log.Error().Msgf("Error in GetPiWallet: %v", err)
			continue
		}

		l := log.Info().
			Str("\nSeed", phrase).
			Str("\nPublic Address", publicAddress)

		// Retrieve account balance
		account, err := GetAccountBalance(publicAddress, horizonURL)
		if err != nil {
			log.Debug().Err(err).Msg("Error in GetAccountBalance")
			continue
		}

		balance, err := account.GetNativeBalance()
		if err == nil {
			l.Str("\nBalance", balance)
		}

		l.Msg("✅  Valid Seed Phrase found")
		found = true
	}

	if !found {
		log.Error().Msg("❌  No valid Seed Phrase found.")
	}
}
