package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// 1) 建立连接
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/xxxx")
	if err != nil {
		log.Fatalf("[ERR] ethclient.Dial: %v", err)
	}
	defer client.Close()

	account := common.HexToAddress("0x25836239F7b632635F815689389C537133248edb")

	// 用统一的超时上下文，避免长时间阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2) 最新余额（nil block）
	latestWei, err := client.BalanceAt(ctx, account, nil)
	if err != nil {
		log.Fatalf("[ERR] BalanceAt(latest): %v", err)
	}
	printBalance("Latest", account, nil, latestWei)

	// 3) 指定区块余额
	blockNumber := big.NewInt(5532993)
	atWei, err := client.BalanceAt(ctx, account, blockNumber)
	if err != nil {
		log.Fatalf("[ERR] BalanceAt(block=%s): %v", blockNumber.String(), err)
	}
	printBalance("At Block", account, blockNumber, atWei)

	// 4) 待处理余额
	pendingWei, err := client.PendingBalanceAt(ctx, account)
	if err != nil {
		log.Fatalf("[ERR] PendingBalanceAt: %v", err)
	}
	printBalance("Pending", account, nil, pendingWei)
}

// printBalance 统一打印：标签、地址、区块、高精度与简洁 ETH、以及原始 wei。
func printBalance(tag string, addr common.Address, blockNumber *big.Int, wei *big.Int) {
	ethPrecise18 := weiToEth(wei, 18) // 精确显示 18 位小数
	ethPretty6 := weiToEth(wei, 6)    // 常用显示 6 位小数

	if blockNumber != nil {
		fmt.Printf("[%s] address=%s  block=%s\n", tag, addr.Hex(), blockNumber.String())
	} else {
		fmt.Printf("[%s] address=%s\n", tag, addr.Hex())
	}

	fmt.Printf("  - balance(wei): %s\n", wei.String())
	fmt.Printf("  - balance(ETH, precise 18dp): %s\n", ethPrecise18)
	fmt.Printf("  - balance(ETH, pretty 6dp):   %s\n\n", ethPretty6)
}

// weiToEth 将 wei 转为 ETH 的字符串表示，scale 指定小数位数（例如 18 或 6）
func weiToEth(wei *big.Int, scale int) string {
	if wei == nil {
		return "0"
	}
	// denom = 10^18
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	// f = wei / 10^18，使用 big.Float 保证大数精度
	fWei := new(big.Float).SetInt(wei)
	fDen := new(big.Float).SetInt(denom)
	fEth := new(big.Float).Quo(fWei, fDen)

	// 返回固定小数位的字符串
	return fEth.Text('f', scale)
}
