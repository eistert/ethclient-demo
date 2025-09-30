// query_block.go
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const defaultRPC = "https://sepolia.infura.io/v3/<YOUR_INFURA_PROJECT_ID>"

func main() {
	rpcURL := getenv("SEPOLIA_RPC", defaultRPC)
	if rpcURL == defaultRPC {
		fmt.Println("[hint] set SEPOLIA_RPC env for convenience")
	}

	if len(os.Args) < 2 {
		log.Fatalf("usage: go run query_block.go <blockNumber>")
	}

	// 1) 连接 RPC
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, rpcURL)
	mustOK("ethclient.Dial", err)
	defer client.Close()

	// 2) 解析区块号
	n := new(big.Int)
	_, ok := n.SetString(os.Args[1], 10)
	if !ok {
		log.Fatalf("invalid block number: %s", os.Args[1])
	}

	// 3) 查询区块
	block, err := client.BlockByNumber(ctx, n)
	mustOK("BlockByNumber", err)

	// 4) 打印结果（优化版）
	fmt.Println("[Block/info]")
	fmt.Printf("  rpc:        %s\n", rpcURL)
	fmt.Printf("  number:     %d\n", block.NumberU64())
	fmt.Printf("区块的哈希  hash: %s (%s)\n", block.Hash().Hex(), shortHash(block.Hash()))
	fmt.Printf("时间戳 time:       %s\n", time.Unix(int64(block.Time()), 0).Format(time.RFC3339))
	fmt.Printf("交易数量  txs:        %d\n", len(block.Transactions()))

	if coinbase := block.Coinbase(); (coinbase != common.Address{}) {
		fmt.Printf("  proposer:   %s (%s)\n", coinbase.Hex(), shortStr(coinbase.Hex()))
	}

	if baseFee := block.BaseFee(); baseFee != nil {
		fmt.Printf("  baseFee:    %s wei\n", baseFee.String())
	}

	fmt.Println("[Done]")
}

func mustOK(tag string, err error) {
	if err != nil {
		log.Fatalf("[ERR] %s: %v", tag, err)
	}
}
func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func shortHash(h common.Hash) string {
	return shortHexStr(h.Hex())
}
func shortHexStr(s string) string {
	if len(s) <= 12 {
		return s
	}
	return fmt.Sprintf("%s...%s", s[:8], s[len(s)-4:])
}

func shortStr(s string) string {
	if len(s) <= 12 {
		return s
	}
	return fmt.Sprintf("%s...%s", s[:6], s[len(s)-4:])
}
