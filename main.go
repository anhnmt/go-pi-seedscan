package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/pflag"
	"github.com/tyler-smith/go-bip39"
)

type Config struct {
	SeedPhrase     string
	Bip39Wordlist  []string
	MaxWordMissing uint
}

var cfg Config

func main() {
	pflag.StringVar(&cfg.SeedPhrase, "s", "", "your seed phrase")
	pflag.StringSliceVar(&cfg.Bip39Wordlist, "l", bip39.GetWordList(), "wordlist seed phrase")
	pflag.UintVar(&cfg.MaxWordMissing, "m", 10, "max word missing")
	pflag.Parse()

	recoverSeedPhrase(cfg.SeedPhrase)
}

// Đệ quy thử tất cả tổ hợp từ bị thiếu
func recoverMissingWords(words []string, missingIndexes []int, index int, wg *sync.WaitGroup, results chan<- string) {
	if index >= len(missingIndexes) {
		// Khi đã điền hết từ bị thiếu, kiểm tra Seed Phrase
		testPhrase := strings.Join(words, " ")
		if bip39.IsMnemonicValid(testPhrase) {
			results <- testPhrase
		}
		return
	}

	for _, word := range cfg.Bip39Wordlist {
		words[missingIndexes[index]] = word
		recoverMissingWords(words, missingIndexes, index+1, wg, results)
	}
}

// Hàm chính để khôi phục Seed Phrase
func recoverSeedPhrase(seedPhrase string) {
	words := strings.Split(seedPhrase, " ")

	// Nếu nhập đủ 24 từ và không có dấu "?", kiểm tra ngay
	if len(words) == 24 && !strings.Contains(seedPhrase, "?") {
		if bip39.IsMnemonicValid(seedPhrase) {
			fmt.Println("✅ Seed Phrase hợp lệ!")
		} else {
			fmt.Println("❌ Seed Phrase không hợp lệ!")
		}
		return
	}

	// Tìm vị trí các từ bị thiếu
	var missingIndexes []int
	for i, word := range words {
		if word == "?" {
			missingIndexes = append(missingIndexes, i)
		}
	}

	// Kiểm tra số lượng từ bị thiếu
	if len(missingIndexes) == 0 {
		fmt.Println("Không có từ nào bị thiếu.")
		return
	} else if len(missingIndexes) > 10 {
		fmt.Println("🚨 Hiện chỉ hỗ trợ khôi phục tối đa 10 từ.")
		return
	}

	// Kênh để nhận kết quả từ Goroutines
	results := make(chan string, 10)
	var wg sync.WaitGroup
	wg.Add(1)

	// Chạy đệ quy để thử tất cả tổ hợp
	go func() {
		defer wg.Done()
		recoverMissingWords(words, missingIndexes, 0, &wg, results)
	}()

	// Goroutine để đóng kênh khi hoàn thành
	go func() {
		wg.Wait()
		close(results)
	}()

	// Hiển thị các Seed Phrase hợp lệ tìm thấy
	found := false
	for phrase := range results {
		found = true
		fmt.Println("🔹 Seed Phrase hợp lệ tìm thấy:", phrase)
	}

	if !found {
		fmt.Println("❌ Không tìm thấy Seed Phrase hợp lệ.")
	}
}
