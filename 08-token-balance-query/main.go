package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	token "example.com/ethclient-demo/08-token-balance-query/erc20" // for demo: 由 abigen 生成的本地包
)

func main() {
	// 1) 连接以太坊节点
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/xxx")
	if err != nil {
		log.Fatalf("[ERR] ethclient.Dial: %v", err)
	}
	defer client.Close()

	// 2) 合约与账户地址
	tokenAddr := common.HexToAddress("0xfadea654ea83c00e5003d2ea15c59830b65471c0")
	account := common.HexToAddress("0x25836239F7b632635F815689389C537133248edb")

	// 3) 生成实例（带超时上下文）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	inst, err := token.NewErc20(tokenAddr, client)
	if err != nil {
		log.Fatalf("[ERR] NewToken: %v", err)
	}

	// 4) 读取代币元信息
	name, err := inst.Name(&bind.CallOpts{Context: ctx})
	if err != nil {
		log.Fatalf("[ERR] Name(): %v", err)
	}
	symbol, err := inst.Symbol(&bind.CallOpts{Context: ctx})
	if err != nil {
		log.Fatalf("[ERR] Symbol(): %v", err)
	}
	decimals, err := inst.Decimals(&bind.CallOpts{Context: ctx})
	if err != nil {
		log.Fatalf("[ERR] Decimals(): %v", err)
	}

	// 5) 读取余额（wei）
	balWei, err := inst.BalanceOf(&bind.CallOpts{Context: ctx}, account)
	if err != nil {
		log.Fatalf("[ERR] BalanceOf(%s): %v", account.Hex(), err)
	}

	// 6) 统一打印
	printTokenHeader(name, symbol, decimals, tokenAddr)
	printBalance("Holder", account, balWei, decimals, symbol)
}

// ======== 打印与格式化工具 ========

// 头部信息：代币名、符号、小数位、合约地址
func printTokenHeader(name, symbol string, decimals uint8, tokenAddr common.Address) {
	fmt.Printf("[Token]\n")
	fmt.Printf("  - name:        %s\n", name)
	fmt.Printf("  - symbol:      %s\n", symbol)
	fmt.Printf("  - decimals:    %d\n", decimals)
	fmt.Printf("  - contract:    %s (%s)\n\n", tokenAddr.Hex(), shortHex(tokenAddr.Hex()))
}

// 余额打印：显示地址、wei、十进制（精确与易读）
func printBalance(tag string, addr common.Address, wei *big.Int, decimals uint8, symbol string) {
	precise := weiToDecimalString(wei, decimals, int(decimals)) // 精确：按代币 decimals 位
	prettyScale := 6
	if int(decimals) < prettyScale {
		prettyScale = int(decimals)
	}
	pretty := weiToDecimalString(wei, decimals, prettyScale) // 易读：默认 6 位（不足则取 decimals）

	fmt.Printf("[%s] address: %s (%s)\n", tag, addr.Hex(), shortHex(addr.Hex()))
	fmt.Printf("  - balance(wei):           %s\n", wei.String())
	fmt.Printf("  - balance(%s, precise):   %s\n", symbol, precise)
	fmt.Printf("  - balance(%s, pretty %ddp): %s\n\n", symbol, prettyScale, pretty)
}

// 将 wei 转换为十进制字符串（scale 是输出小数位数）
func weiToDecimalString(wei *big.Int, decimals uint8, scale int) string {
	if wei == nil {
		return "0"
	}
	// denom = 10^decimals（用 big.Int 精确表示）
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// fEth = wei / 10^decimals
	fWei := new(big.Float).SetInt(wei)
	fDen := new(big.Float).SetInt(denom)
	fEth := new(big.Float).Quo(fWei, fDen)

	// 固定小数位输出
	return fEth.Text('f', scale)
}

// 简写 0x 地址：0x1234...ABCD
func shortHex(hexAddr string) string {
	if len(hexAddr) <= 12 {
		return hexAddr
	}
	return fmt.Sprintf("%s...%s", hexAddr[:6], hexAddr[len(hexAddr)-4:])
}
