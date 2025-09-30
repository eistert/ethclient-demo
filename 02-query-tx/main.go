package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 建议：把你的 Alchemy/Infura/QuickNode 的 RPC URL 放到环境变量里更安全
const rpcURL = "https://eth-sepolia.g.alchemy.com/v2/xxx"

func main() {
	// 统一给所有 RPC 调用一个超时（生产中建议配合重试）
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cli, err := ethclient.DialContext(ctx, rpcURL)
	must(err, "dial rpc")
	defer cli.Close()
	fmt.Println("✅ Connected:", rpcURL)

	// 1) 获取链 ID（签名恢复 sender 需要）
	chainID, err := cli.ChainID(ctx)
	must(err, "chain id")
	fmt.Println("🔗 ChainID:", chainID)

	// 2) 查一个指定高度的完整区块
	blockNumber := big.NewInt(5_671_744) // 可替换为你想查的高度
	blk, err := cli.BlockByNumber(ctx, blockNumber)
	must(err, "block by number")
	fmt.Printf("\n📦 Block  #%v  hash=%s  time=%v  txs=%d  gasUsed=%d\n",
		blk.Number(),
		blk.Hash(),
		time.Unix(int64(blk.Time()), 0),
		len(blk.Transactions()),
		blk.GasUsed())

	// 3) 演示：遍历区块里的第一笔交易并打印关键信息
	for i, tx := range blk.Transactions() {
		fmt.Printf("\n—— Transaction #%d ———————————————————————————————\n", i)
		printTxBasic(tx)

		// (3.1) 恢复交易发送者（自动适配 Legacy/EIP-1559）
		signer := types.LatestSignerForChainID(chainID)
		from, err := types.Sender(signer, tx)
		if err != nil {
			fmt.Println("  sender: <recover failed>", err)
		} else {
			fmt.Println("  from:  ", from.Hex())
		}

		// (3.2) 查询这笔交易的收据，拿到执行状态、日志数量、实际成交 gas 价格
		rcp, err := cli.TransactionReceipt(ctx, tx.Hash())
		must(err, "tx receipt")
		status := "FAIL"
		if rcp.Status == types.ReceiptStatusSuccessful {
			status = "SUCCESS"
		}
		fmt.Printf("  receipt: status=%s  block=%d  gasUsed=%d  logs=%d\n",
			status, rcp.BlockNumber.Uint64(), rcp.GasUsed, len(rcp.Logs))

		// EffectiveGasPrice：交易打包时实际支付的每 gas 单价（EIP-1559/Legacy 都有值）
		if rcp.EffectiveGasPrice != nil {
			totalFee := new(big.Int).Mul(rcp.EffectiveGasPrice, big.NewInt(int64(rcp.GasUsed)))
			fmt.Printf("  effectiveGasPrice: %s wei (≈ %f ETH)\n",
				rcp.EffectiveGasPrice.String(), weiToEth(rcp.EffectiveGasPrice))
			fmt.Printf("  totalFee:          %s wei (≈ %f ETH)\n",
				totalFee.String(), weiToEth(totalFee))
		}

		// 只示范一笔，演示明白即可；去掉 break 可遍历所有
		break
	}

	// 4) 不拉全块也能拿交易数量：通过区块哈希配合 TransactionCount
	blockHash := common.HexToHash("0xae713dea1419ac72b928ebe6ba9915cd4fc1ef125a606f90f5e783c47cb1a4b5")
	cnt, err := cli.TransactionCount(ctx, blockHash)
	must(err, "tx count by block hash")
	fmt.Printf("\n🧮 TxCount of block %s => %d\n", short(blockHash.Hex(), 16), cnt)

	// 5) 演示：按“区块内索引”读取第 0 笔交易
	if cnt > 0 {
		t0, err := cli.TransactionInBlock(ctx, blockHash, 0)
		must(err, "tx in block(0)")
		fmt.Printf("🔎 First tx in block %s => %s\n", short(blockHash.Hex(), 16), t0.Hash().Hex())
	}

	// 6) 演示：按哈希直查某笔交易
	txHash := common.HexToHash("0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5")
	tx, isPending, err := cli.TransactionByHash(ctx, txHash)
	must(err, "tx by hash")
	fmt.Printf("\n🔗 TransactionByHash(%s)\n", short(txHash.Hex(), 16))
	fmt.Println("  pending:", isPending)
	printTxBasic(tx)

	fmt.Println("\n✅ Done.")
}

// —— 工具函数 ——

// 友好打印交易的基础信息（兼容 Legacy 和 EIP-1559）
func printTxBasic(tx *types.Transaction) {
	to := "<contract-creation>"
	if tx.To() != nil {
		to = tx.To().Hex()
	}
	fmt.Println("tx  hash:    ", tx.Hash().Hex())
	fmt.Println("tx nonce:   ", tx.Nonce())
	fmt.Println("tx  to:      ", to)
	fmt.Printf("tx  value:    %s wei (≈ %f ETH)\n", tx.Value().String(), weiToEth(tx.Value()))
	fmt.Println("tx  gasLimit:", tx.Gas())

	// Legacy 交易：GasPrice 有值；EIP-1559：优先看 TipCap/FeeCap
	if gp := tx.GasPrice(); gp != nil {
		fmt.Printf("  gasPrice: %s wei (legacy)\n", gp.String())
	}
	if tip := tx.GasTipCap(); tip != nil {
		fmt.Printf("  tipCap:   %s wei\n", tip.String())
	}
	if fee := tx.GasFeeCap(); fee != nil {
		fmt.Printf("  feeCap:   %s wei\n", fee.String())
	}

	// Data 只打印长度，避免刷屏
	fmt.Printf("  data:     %d bytes\n", len(tx.Data()))
}

func must(err error, where string) {
	if err != nil {
		log.Fatalf("❌ %s: %v", where, err)
	}
}

func weiToEth(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}
	f := new(big.Float).SetInt(wei)                            // wei -> big.Float
	eth := new(big.Float).Quo(f, big.NewFloat(math.Pow10(18))) // / 1e18
	val, _ := eth.Float64()
	return val
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
