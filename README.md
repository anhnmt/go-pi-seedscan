# Pi Wallet Seed Phrase Recovery Tool

This tool helps recover a **Pi Network wallet seed phrase** when one or more words are missing. It attempts to reconstruct the correct mnemonic by testing possible word combinations.

## 🚀 Features
- Supports recovery of up to **10 missing words** (customizable).
- Uses the **BIP-39 standard** for mnemonic validation.
- Works with **Pi Network's Horizon API** to check account balance.
- Supports both **Testnet** and **Mainnet**.
- High-performance parallel processing with **Go routines**.

## 📥 Installation

### **Prerequisites**
- Go **1.24+** installed ([Download Go](https://go.dev/doc/install))
- Git installed ([Download Git](https://git-scm.com/downloads))

### **Clone and Build**
```sh
git clone https://github.com/anhnmt/go-pi-seedscan.git
cd go-pi-seedscan
go build -o pi-seedscan
```

## 🛠️ Usage

### **Command-line Options**
```sh
./pi-seedscan --seed "word1 word2 ? word4 ..." [OPTIONS]
```
| Flag            | Short | Description                                   | Default |
|----------------|-------|-----------------------------------------------|---------|
| `--seed`       | `-s`  | Seed phrase with missing words (`?` as placeholder). | **(Required)** |
| `--max_word`   | `-m`  | Maximum number of missing words to recover.   | `5`     |
| `--batch`      | `-b`  | Batch size for processing combinations.       | `10`    |
| `--testnet`    | `-t`  | Use Pi Testnet instead of Mainnet.           | `false` |
| `--debug`      | `-d`  | Enable debug mode for more logs.              | `false` |

### **Example Usage**
```sh
# Recover a seed phrase with one missing word
./pi-seedscan -s "word1 word2 ? word4 word5 ..."

# Recover a phrase with 3 missing words on Testnet
./pi-seedscan -s "word1 ? ? word4 ?" -b 15 -t

# Enable debug mode for detailed logs
./pi-seedscan -s "word1 word2 ? word4 ..." -d
```

## ⚙️ How It Works
1. **Splits the seed phrase** into individual words.
2. **Identifies missing positions** marked by `?`.
3. **Recursively tests** all possible words in those positions.
4. **Validates each phrase** using the **BIP-39 standard**.
5. **Checks account balance** on Pi Network's blockchain.

## 📜 Example Output

```
✅ Valid Seed Phrase found
Seed=word1 word2 word3 word4 ... word24
Public Address=GBJ2HPQXWQNEMYRXEZIXYSUUM7SBDGFR5EYP3CNGNGSXXQHARCSKF2CY
Balance=314.159265 Pi
```

## 🏗️ Development & Contribution

### **1️⃣ Clone the Repository**
```sh
git clone https://github.com/anhnmt/go-pi-seedscan.git
cd go-pi-seedscan
```

### **2️⃣ Install Dependencies**
```sh
go mod tidy
```

### **3️⃣ Run in Debug Mode**
```sh
go run main.go -s "word1 word2 ? word4 ..." -d
```

### **4️⃣ Contribute**
- **Report bugs** via [GitHub Issues](https://github.com/anhnmt/go-pi-seedscan/issues).
- **Submit a Pull Request (PR)** for improvements.

## ⚠️ Disclaimer
This tool is provided **as is**, and the authors are **not responsible** for any loss of funds. Always **backup your seed phrase securely**.

## 📜 License
This project is licensed under the **MIT License**. See [`LICENSE`](./LICENSE) for details.  
