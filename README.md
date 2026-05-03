# go-pi-seedscan

Recover missing words in a **Pi Network** wallet seed phrase. The tool brute-forces unknown positions using BIP-39 validation, then verifies recovered wallets against the Pi Horizon API.

## How it works

1. You supply your 24-word seed phrase with `?` in place of each missing word.
2. The tool fans out across all CPU cores, testing every BIP-39 word at each `?` position.
3. A fast SHA-256 checksum pre-filter discards ~99.6 % of invalid candidates before full validation.
4. Each valid mnemonic is derived into a Pi wallet address and checked on-chain for an active account.

> **Performance note** — 1 missing word (2 048 candidates) completes in under a second. 2 missing words (~4 M) takes seconds. Each additional word multiplies the search space by 2 048×, so 3+ missing words can take a long time.

## Installation

**Requirements:** Go 1.26+

```sh
go install github.com/anhnmt/go-pi-seedscan@latest
```

Or build from source:

```sh
git clone https://github.com/anhnmt/go-pi-seedscan.git
cd go-pi-seedscan
go build -o go-pi-seedscan
```

## Usage

```sh
./go-pi-seedscan -s "word1 word2 ? word4 ... word24"
```

### Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--seed` | `-s` | *(required)* | Seed phrase — use `?` for each missing word |
| `--max-word` | `-m` | `5` | Maximum missing words allowed |
| `--workers` | `-w` | CPU cores | Parallel worker count |
| `--testnet` | `-t` | `false` | Query Testnet instead of Mainnet |
| `--stop-first` | `-f` | `true` | Stop after the first wallet with balance is found |
| `--debug` | `-d` | `false` | Verbose logging |

### Examples

```sh
# One missing word
./go-pi-seedscan -s "word1 word2 ? word4 word5 ... word24"

# Three missing words on Testnet, 16 workers
./go-pi-seedscan -s "word1 ? ? word4 ? word6 ... word24" -t -w 16

# Validate a complete phrase (no ? present)
./go-pi-seedscan -s "word1 word2 word3 ... word24"
```

### Example output

```
12:00:00 INF Starting recovery missing_words=1 combinations=2048 workers=8
12:00:01 INF ✅ Valid Seed Phrase with active account
         seed=word1 word2 word3 ... word24
         address=GBJ2HPQXWQNEMYRXEZIXYSUUM7SBDGFR5EYP3CNGNGSXXQHARCSKF2CY
         balance=314.159265
12:00:01 INF Done elapsed=1.23s
```

## Architecture

```
main
 ├─ initWordList()          build word→index map once (O(1) lookups)
 ├─ RecoverSeedPhrase()
 │   ├─ fan-out             split wordList across N workers
 │   │   └─ recoverInner()  recursive depth-first search (own slice copy)
 │   │       └─ fastChecksumValid()  SHA-256 checksum pre-filter
 │   └─ processResults()
 │       └─ bounded API pool (10 concurrent Horizon requests, 10 s timeout)
 └─ context.WithCancel      early termination on first hit
```

Key design decisions:

- **No shared mutable state** — each worker deep-copies the word slice, eliminating data races.
- **Checksum pre-filter** — converts words→entropy bits and verifies the SHA-256 checksum directly, avoiding string allocation and full mnemonic parsing for invalid candidates.
- **Bounded API concurrency** — at most 10 in-flight Horizon requests with per-request timeouts prevent rate-limiting and hangs.
- **Early termination** — `context.WithCancel` propagates to all workers when `--stop-first` is set.

## Contributing

```sh
git clone https://github.com/anhnmt/go-pi-seedscan.git
cd go-pi-seedscan
go mod tidy
go run main.go -s "word1 word2 ? word4 ..." -d
```

Bug reports and PRs welcome via [GitHub Issues](https://github.com/anhnmt/go-pi-seedscan/issues).

## Disclaimer

This tool is provided as-is. The authors are not responsible for any loss of funds. Always store your seed phrase securely offline.

## License

[MIT](./LICENSE)