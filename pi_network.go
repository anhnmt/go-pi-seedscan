package main

import (
	"encoding/hex"
	"fmt"

	"github.com/stellar/go/exp/crypto/derivation"
	"github.com/stellar/go/keypair"
	"github.com/tyler-smith/go-bip39"
)

// Tạo địa chỉ ví Pi từ Seed Phrase với đường dẫn linh hoạt
func getPiWallet(seedPhrase, derivationPath string) (string, string, error) {
	// Tạo seed từ mnemonic
	seed := bip39.NewSeed(seedPhrase, "")

	// Dẫn xuất key theo đường dẫn được truyền vào
	derivedKey, err := derivation.DeriveForPath(derivationPath, seed)
	if err != nil {
		return "", "", fmt.Errorf("Lỗi dẫn xuất key: %v", err)
	}

	// Lấy 32 byte đầu tiên làm seed cho Ed25519
	var ed25519Seed [32]byte
	copy(ed25519Seed[:], derivedKey.Key[:32]) // Ép kiểu về [32]byte

	// Chuyển về hex để kiểm tra
	privateKeyHex := hex.EncodeToString(ed25519Seed[:])

	// Tạo Stellar Keypair (Pi Network)
	kp, err := keypair.FromRawSeed(ed25519Seed)
	if err != nil {
		return "", "", fmt.Errorf("Lỗi tạo keypair Stellar: %v", err)
	}

	return privateKeyHex, kp.Address(), nil
}
