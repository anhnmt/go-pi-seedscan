package main

import (
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/tyler-smith/go-bip39"
)

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

	for _, word := range bip39.GetWordList() {
		words[missingIndexes[index]] = word
		recoverMissingWords(words, missingIndexes, index+1, wg, results)
	}
}

// Hàm chính để khôi phục Seed Phrase
func recoverSeedPhrase(cfg Config) {
	words := strings.Split(cfg.SeedPhrase, " ")

	// Nếu nhập đủ 24 từ và không có dấu "?", kiểm tra ngay
	if len(words) == 24 && !strings.Contains(cfg.SeedPhrase, "?") {
		if bip39.IsMnemonicValid(cfg.SeedPhrase) {
			log.Info().Msg("✅ Seed Phrase hợp lệ!")
		} else {
			log.Error().Msg("❌ Seed Phrase không hợp lệ!")
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
		log.Info().Msg("Không có từ nào bị thiếu.")
		return
	} else if len(missingIndexes) > cfg.MaxWordMissing {
		log.Error().Msg("🚨 Hiện chỉ hỗ trợ khôi phục tối đa 10 từ.")
		return
	}

	// Kênh để nhận kết quả từ Goroutines
	results := make(chan string, cfg.BatchSize)
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

		_, publicAddress, err := getPiWallet(phrase, cfg.DerivationPath)
		if err != nil {
			log.Fatal().Msgf("Lỗi: %v", err)
		}

		log.Info().
			Str("\nseed_phrase", phrase).
			Str("\npublic_address", publicAddress).
			Msg("✅  Seed Phrase hợp lệ tìm thấy")
	}

	if !found {
		log.Error().Msg("❌ Không tìm thấy Seed Phrase hợp lệ.")
	}
}
