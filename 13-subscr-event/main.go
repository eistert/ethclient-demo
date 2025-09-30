package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	wsURL    = "wss://eth-sepolia.g.alchemy.com/v2/xxx" // Rinkeby 已下线，用 Sepolia/Goerli 等
	storeABI = `[{"inputs":[{"internalType":"string","name":"_version","type":"string"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"key","type":"bytes32"},{"indexed":false,"internalType":"bytes32","name":"value","type":"bytes32"}],"name":"ItemSet","type":"event"},{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"items","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"key","type":"bytes32"},{"internalType":"bytes32","name":"value","type":"bytes32"}],"name":"setItem","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"version","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}]`
)

func main() {
	// 1) 建立 WS 连接
	client, err := ethclient.Dial(wsURL)
	mustOK("ethclient.Dial(WS)", err)
	defer client.Close()

	contract := common.HexToAddress("0x2958d15bc5b64b11Ec65e623Ac50C198519f8742")
	query := ethereum.FilterQuery{Addresses: []common.Address{contract}}

	logsCh := make(chan types.Log, 64)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logsCh)
	mustOK("SubscribeFilterLogs", err)

	// 准备 ABI 与事件签名
	parsed, err := abi.JSON(strings.NewReader(storeABI))
	mustOK("abi.JSON", err)
	sigHash := crypto.Keccak256Hash([]byte("ItemSet(bytes32,bytes32)"))

	fmt.Println("[Events/subscribe]")
	fmt.Printf("  ws:       %s\n", wsURL)
	fmt.Printf("  contract: %s (%s)\n", contract.Hex(), short(contract.Hex()))
	fmt.Printf("  status:   subscribed, waiting for new logs...\n")

	for {
		select {
		case err := <-sub.Err():
			log.Fatalf("[ERR] subscription: %v", err)

		case lg := <-logsCh:
			fmt.Println("\n[Log]")
			fmt.Printf("  block:   %d (%s)\n", lg.BlockNumber, lg.BlockHash.Hex())
			fmt.Printf("  tx:      %s\n", lg.TxHash.Hex())

			if len(lg.Topics) == 0 || lg.Topics[0] != sigHash {
				fmt.Printf("  note:    non-ItemSet topic0=%v\n", lg.Topics)
				continue
			}
			// 解码 value（非 indexed）
			var data struct{ Value [32]byte }
			if err := parsed.UnpackIntoInterface(&data, "ItemSet", lg.Data); err != nil {
				log.Printf("[WARN] unpack ItemSet data: %v", err)
				continue
			}
			// 读取 indexed 的 key（topics[1]）
			key := ""
			if len(lg.Topics) > 1 {
				key = lg.Topics[1].Hex()
			}

			fmt.Printf("  topic0:  %s\n", lg.Topics[0].Hex())
			if key != "" {
				fmt.Printf("  key:     %s\n", key)
			}
			fmt.Printf("  value:   0x%s\n", hex.EncodeToString(data.Value[:]))
		}
	}
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
