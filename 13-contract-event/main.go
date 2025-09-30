package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rpcURL   = "https://eth-sepolia.g.alchemy.com/v2/fBR8OwccYIS5h7DcKaQ53"
	timeout  = 20 * time.Second
	storeABI = `[{"inputs":[{"internalType":"string","name":"_version","type":"string"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"key","type":"bytes32"},{"indexed":false,"internalType":"bytes32","name":"value","type":"bytes32"}],"name":"ItemSet","type":"event"},{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"items","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"key","type":"bytes32"},{"internalType":"bytes32","name":"value","type":"bytes32"}],"name":"setItem","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"version","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}]`
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := ethclient.Dial(rpcURL)
	mustOK("ethclient.Dial", err)
	defer client.Close()

	contract := common.HexToAddress("0x2958d15bc5b64b11Ec65e623Ac50C198519f8742")

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(6920583), // 示例：起始区块
		// ToBlock: big.NewInt(6922000), // 可选：结束区块
		Addresses: []common.Address{contract},
		// Topics: [][]common.Hash{ {eventSigHash}, {indexedKey?}, ... },
	}

	logs, err := client.FilterLogs(ctx, query)
	mustOK("FilterLogs", err)

	parsed, err := abi.JSON(strings.NewReader(storeABI))
	mustOK("abi.JSON", err)

	// 事件签名与 topics[0]
	sig := []byte("ItemSet(bytes32,bytes32)")
	sigHash := crypto.Keccak256Hash(sig)

	fmt.Println("[Events/query]")
	fmt.Printf("  rpc:       %s\n", rpcURL)
	fmt.Printf("  contract:  %s (%s)\n", contract.Hex(), short(contract.Hex()))
	fmt.Printf("  fromBlock: %v\n", query.FromBlock)
	fmt.Printf("  toBlock:   %v\n", query.ToBlock)
	fmt.Printf("  logs:      %d\n", len(logs))
	fmt.Printf("  topic0:    %s  // keccak(\"%s\")\n", sigHash.Hex(), string(sig))

	for i, lg := range logs {
		fmt.Printf("\n#%d\n", i+1)
		fmt.Printf("  block:   %d (%s)\n", lg.BlockNumber, lg.BlockHash.Hex())
		fmt.Printf("  tx:      %s\n", lg.TxHash.Hex())

		// 安全获取 topics
		if len(lg.Topics) == 0 || lg.Topics[0] != sigHash {
			fmt.Printf("  warn:    unexpected topic0=%v\n", lg.Topics)
			continue
		}

		// 解析非 indexed 的 Data：本例中只有 value(bytes32)
		var ev struct {
			Value [32]byte
		}
		err := parsed.UnpackIntoInterface(&ev, "ItemSet", lg.Data)
		mustOK("UnpackIntoInterface(ItemSet)", err)

		// 解析 indexed 的 key：topics[1]
		var keyHex string
		if len(lg.Topics) > 1 {
			// ABI 把 bytes32 编入 topic 时即 32字节；取其 Hex 展示
			keyHex = lg.Topics[1].Hex()
		}

		fmt.Printf("  topics0: %s\n", lg.Topics[0].Hex())
		if keyHex != "" {
			fmt.Printf("  key:     %s\n", keyHex)
		}
		fmt.Printf("  value:   0x%s\n", hex.EncodeToString(ev.Value[:]))
	}

	fmt.Println("\n[Done]")
}

func mustOK(tag string, err error) {
	if err != nil {
		log.Fatalf("[ERR] %s: %v", tag, err)
	}
}
func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return fmt.Sprintf("%s...%s", s[:6], s[len(s)-4:])
}
