package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/tools/stellar-hd-wallet/crypto/derivation"
	"github.com/tyler-smith/go-bip39"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	DefaultMainNetURL = "https://api.mainnet.minepi.com"
	DefaultTestNetURL = "https://api.testnet.minepi.com"
	DerivationPath    = "m/44'/314159'/0'"

	MaxAPIConcurrency = 10               // max concurrent Horizon API calls
	APITimeout        = 10 * time.Second // per-request timeout for Horizon
)

// ─── Config ──────────────────────────────────────────────────────────────────

type Config struct {
	SeedPhrase     string
	MaxWordMissing int
	Workers        int
	Testnet        bool
	Debug          bool
	StopOnFirst    bool // stop after the first wallet with balance is found
}

func parseConfig() Config {
	var cfg Config
	pflag.StringVarP(&cfg.SeedPhrase, "seed", "s", "", "Seed phrase (use ? for missing words)")
	pflag.IntVarP(&cfg.MaxWordMissing, "max-word", "m", 5, "Max missing words allowed (0-24)")
	pflag.IntVarP(&cfg.Workers, "workers", "w", runtime.NumCPU(), "Number of parallel workers")
	pflag.BoolVarP(&cfg.Testnet, "testnet", "t", false, "Use Testnet instead of Mainnet")
	pflag.BoolVarP(&cfg.Debug, "debug", "d", false, "Enable debug logging")
	pflag.BoolVarP(&cfg.StopOnFirst, "stop-first", "f", true, "Stop after first wallet with balance")
	pflag.Parse()
	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.SeedPhrase == "" {
		return fmt.Errorf("seed phrase cannot be empty (use -s)")
	}
	if cfg.MaxWordMissing < 0 || cfg.MaxWordMissing > 24 {
		return fmt.Errorf("max-word must be between 0 and 24")
	}
	if cfg.Workers <= 0 {
		return fmt.Errorf("workers must be > 0")
	}
	return nil
}

// ─── Logger ──────────────────────────────────────────────────────────────────

func initLogger(debug bool) {
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.InterfaceMarshalFunc = sonic.Marshal
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	log.Logger = zerolog.New(&zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}).With().Timestamp().Caller().Logger()
}

// ─── Horizon helpers ─────────────────────────────────────────────────────────

func horizonURL(testnet bool) string {
	if testnet {
		return DefaultTestNetURL
	}
	return DefaultMainNetURL
}

// getAccountBalance fetches the account from Horizon with a context timeout.
func getAccountBalance(ctx context.Context, address, hURL string) (*horizon.Account, error) {
	client := horizonclient.Client{HorizonURL: hURL}
	req := horizonclient.AccountRequest{AccountID: address}

	// horizonclient doesn't accept context natively,
	// so we wrap the call with a deadline-aware goroutine.
	type result struct {
		acc horizon.Account
		err error
	}
	ch := make(chan result, 1)
	go func() {
		acc, err := client.AccountDetail(req)
		ch <- result{acc, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		return &r.acc, nil
	}
}

// ─── Wallet derivation ──────────────────────────────────────────────────────

func derivePiAddress(seedPhrase string) (string, error) {
	seed := bip39.NewSeed(seedPhrase, "")
	dk, err := derivation.DeriveForPath(DerivationPath, seed)
	if err != nil {
		return "", errors.Wrap(err, "derive key")
	}
	var raw [32]byte
	copy(raw[:], dk.Key[:32])
	kp, err := keypair.FromRawSeed(raw)
	if err != nil {
		return "", errors.Wrap(err, "keypair from seed")
	}
	return kp.Address(), nil
}

// ─── BIP-39 fast checksum pre-filter ─────────────────────────────────────────
//
// A BIP-39 mnemonic maps to entropy + checksum.  For a 24-word (256-bit) phrase
// the last 8 bits of the last word encode the SHA-256 checksum of the entropy.
// We can reject ~99.6% of candidates cheaply by verifying the checksum ourselves
// before calling the heavier bip39.IsMnemonicValid (which re-parses the string).
//
// wordListMap is built once at startup for O(1) word→index lookup.

var (
	wordList    []string
	wordListMap map[string]int
)

func initWordList() {
	wordList = bip39.GetWordList()
	wordListMap = make(map[string]int, len(wordList))
	for i, w := range wordList {
		wordListMap[w] = i
	}
}

// fastChecksumValid converts words→entropy and validates the SHA-256 checksum
// without string allocation.  Returns false for any unknown word.
func fastChecksumValid(words []string) bool {
	if len(words) != 24 {
		return false
	}

	// 24 words × 11 bits = 264 bits = 256 entropy + 8 checksum
	// Pack the 11-bit indices into a byte buffer (33 bytes).
	var buf [33]byte
	bitPos := 0
	for _, w := range words {
		idx, ok := wordListMap[w]
		if !ok {
			return false
		}
		// Write 11 bits of idx into buf starting at bitPos.
		for i := 10; i >= 0; i-- {
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			if idx&(1<<i) != 0 {
				buf[byteIdx] |= 1 << bitIdx
			}
			bitPos++
		}
	}

	// First 32 bytes = entropy, last byte = checksum (8 bits).
	entropy := buf[:32]
	checksumByte := buf[32]

	hash := sha256.Sum256(entropy)
	return hash[0] == checksumByte
}

// ─── Recovery engine ─────────────────────────────────────────────────────────

// recoverInner recursively fills missing positions depth-first.
// Each goroutine owns its own `words` slice — no shared mutation.
func recoverInner(words []string, missing []int, depth int, results chan<- string, canceled func() bool) {
	if canceled() {
		return
	}

	if depth >= len(missing) {
		// All missing words filled → validate checksum fast, then full check.
		if fastChecksumValid(words) {
			phrase := strings.Join(words, " ")
			// Double-check with library (handles edge cases).
			if bip39.IsMnemonicValid(phrase) {
				results <- phrase
			}
		}
		return
	}

	pos := missing[depth]
	for _, w := range wordList {
		words[pos] = w
		recoverInner(words, missing, depth+1, results, canceled)
		if canceled() {
			return
		}
	}
}

// RecoverSeedPhrase is the top-level entry point.
func RecoverSeedPhrase(cfg Config) {
	words := strings.Split(cfg.SeedPhrase, " ")

	// ── Fast path: full phrase supplied ──
	if len(words) == 24 && !strings.Contains(cfg.SeedPhrase, "?") {
		if bip39.IsMnemonicValid(cfg.SeedPhrase) {
			log.Info().Msg("✅ Valid Seed Phrase!")
		} else {
			log.Error().Msg("❌ Invalid Seed Phrase!")
		}
		return
	}

	// ── Identify missing positions ──
	var missing []int
	for i, w := range words {
		if w == "?" {
			missing = append(missing, i)
		}
	}

	if len(missing) == 0 {
		log.Info().Msg("No missing words detected.")
		return
	}
	if len(missing) > cfg.MaxWordMissing {
		log.Error().Msgf("🚨 %d missing words exceeds limit of %d.", len(missing), cfg.MaxWordMissing)
		return
	}

	totalCombinations := 1.0
	for i := 0; i < len(missing); i++ {
		totalCombinations *= float64(len(wordList))
	}
	log.Info().
		Int("missing_words", len(missing)).
		Str("combinations", fmt.Sprintf("%.0f", totalCombinations)).
		Int("workers", cfg.Workers).
		Msg("Starting recovery")

	// ── Cancellable context for early termination ──
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan string, 256)

	// ── Fan-out: split first missing position across workers ──
	//
	// Each worker gets a disjoint subset of wordList for the first missing
	// position, so zero contention on the shared `words` slice (each worker
	// gets its own deep copy).
	var wg sync.WaitGroup
	chunkSize := (len(wordList) + cfg.Workers - 1) / cfg.Workers

	for i := 0; i < cfg.Workers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(wordList) {
			end = len(wordList)
		}
		if start >= len(wordList) {
			break
		}

		wg.Add(1)
		go func(subset []string) {
			defer wg.Done()
			// Each goroutine gets its own copy of the words slice.
			localWords := make([]string, len(words))
			copy(localWords, words)

			for _, firstWord := range subset {
				if ctx.Err() != nil {
					return
				}
				localWords[missing[0]] = firstWord
				recoverInner(localWords, missing, 1, results, func() bool {
					return ctx.Err() != nil
				})
			}
		}(wordList[start:end])
	}

	// Close results channel when all workers finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// ── Process results with bounded API concurrency ──
	processResults(ctx, cancel, results, horizonURL(cfg.Testnet), cfg.StopOnFirst)
}

// processResults reads valid phrases from the channel and checks their balance
// with bounded concurrency and optional early termination.
func processResults(
	ctx context.Context,
	cancel context.CancelFunc,
	results <-chan string,
	hURL string,
	stopOnFirst bool,
) {
	sem := make(chan struct{}, MaxAPIConcurrency)
	var found atomic.Bool
	var mu sync.Mutex
	var wgAPI sync.WaitGroup

	for phrase := range results {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{} // acquire slot
		wgAPI.Add(1)
		go func(p string) {
			defer wgAPI.Done()
			defer func() { <-sem }() // release slot

			if ctx.Err() != nil {
				return
			}

			addr, err := derivePiAddress(p)
			if err != nil {
				log.Error().Err(err).Msg("derivePiAddress failed")
				return
			}

			reqCtx, reqCancel := context.WithTimeout(ctx, APITimeout)
			defer reqCancel()

			account, err := getAccountBalance(reqCtx, addr, hURL)
			if err != nil {
				log.Debug().Err(err).Str("address", addr).Msg("balance check skipped")
				return
			}

			balance, _ := account.GetNativeBalance()

			mu.Lock()
			log.Info().
				Str("seed", p).
				Str("address", addr).
				Str("balance", balance).
				Msg("✅ Valid Seed Phrase with active account")
			mu.Unlock()

			found.Store(true)
			if stopOnFirst {
				cancel() // signal all workers to stop
			}
		}(phrase)
	}

	wgAPI.Wait()

	if !found.Load() {
		log.Error().Msg("❌ No valid Seed Phrase with active account found.")
	}
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	cfg := parseConfig()
	initLogger(cfg.Debug)

	if err := validateConfig(cfg); err != nil {
		log.Fatal().Err(err).Msg("invalid config")
	}

	initWordList() // build word→index map once
	start := time.Now()

	RecoverSeedPhrase(cfg)

	log.Info().Dur("elapsed", time.Since(start)).Msg("Done")
}
