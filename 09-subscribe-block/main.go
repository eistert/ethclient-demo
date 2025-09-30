package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// 1) 连接 WebSocket 节点（示例使用 Sepolia；替换为你的实际 WS URL）
	wsURL := "wss://eth-sepolia.g.alchemy.com/v2/fBR8OwccYIS5h7DcKaQ53"
	client, err := ethclient.Dial(wsURL)
	if err != nil {
		log.Fatalf("[ERR] ethclient.Dial: %v", err)
	}
	defer client.Close()
	log.Printf("[OK ] connected: %s", wsURL)

	// 2) 订阅新区块头
	headers := make(chan *types.Header, 16)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatalf("[ERR] SubscribeNewHead: %v", err)
	}
	log.Println("[OK ] subscribed to new heads")

	// Ctrl+C 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-quit:
			log.Println("[EXIT] received interrupt, bye")
			return

		case err := <-sub.Err():
			log.Fatalf("[ERR] subscription error: %v", err)

		case header := <-headers:
			// 简要打印头部信息
			printHeaderBrief(header)

			// 3) 拉取完整区块（为避免阻塞，这里给个短超时）
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			block, err := client.BlockByHash(ctx, header.Hash())
			cancel()
			if err != nil {
				log.Printf("[WARN] BlockByHash(%s) failed: %v", header.Hash().Hex(), err)
				continue
			}

			// 4) 统一格式打印区块详情
			printBlock(block)
		}
	}
}

// ======== 打印工具函数 ========

func printHeaderBrief(h *types.Header) {
	fmt.Printf("\n[NewHead]\n")
	fmt.Printf("  - number:     %s\n", h.Number.String())
	fmt.Printf("  - hash:       %s (%s)\n", h.Hash().Hex(), shortHex(h.Hash().Hex()))
	fmt.Printf("  - parent:     %s (%s)\n", h.ParentHash.Hex(), shortHex(h.ParentHash.Hex()))
}

func printBlock(b *types.Block) {
	t := time.Unix(int64(b.Time()), 0)
	fmt.Printf("[Block]\n")
	fmt.Printf("  - hash:       %s (%s)\n", b.Hash().Hex(), shortHex(b.Hash().Hex()))
	fmt.Printf("  - number:     %d\n", b.Number().Uint64())
	fmt.Printf("  - time:       %s | UTC: %s\n", t.Local().Format(time.RFC3339), t.UTC().Format(time.RFC3339))
	fmt.Printf("  - txs:        %d\n", len(b.Transactions()))
	fmt.Printf("  - gas(used/limit): %d / %d\n", b.GasUsed(), b.GasLimit())
	if bf := b.BaseFee(); bf != nil {
		fmt.Printf("  - baseFee:    %s wei\n", bf.String())
	}
	// 注意：在 PoS 的以太坊主网/测试网中 Nonce 通常无意义，但保留打印以兼容教程
	fmt.Printf("  - nonce:      %v\n\n", b.Nonce())
}

func shortHex(h string) string {
	if len(h) <= 12 {
		return h
	}
	return fmt.Sprintf("%s...%s", h[:8], h[len(h)-6:])
}
