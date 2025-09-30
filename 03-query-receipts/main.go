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
	"github.com/ethereum/go-ethereum/rpc"
)

// 建议把密钥改为环境变量读取；这里为演示方便先写死
const rpcURL = "https://eth-sepolia.g.alchemy.com/v2/fBR8OwccYIS5h7DcKaQ53"

func main() {
	// 统一超时，避免网络抖动时阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, rpcURL)
	must(err, "dial rpc")
	defer client.Close()
	fmt.Println("✅ Connected:", rpcURL)

	// 示例区块高度 & 区块哈希（来自你原始代码）
	blockNumber := big.NewInt(5_671_744)
	blockHash := common.HexToHash("0xae713dea1419ac72b928ebe6ba9915cd4fc1ef125a606f90f5e783c47cb1a4b5")

	// 1) 按区块哈希获取整块收据
	rcptsByHash, err := fetchReceiptsByHash(ctx, client, blockHash)
	must(err, "block receipts by hash")
	fmt.Printf("📦 Receipts(by hash %s): %d\n", short(blockHash.Hex(), 16), len(rcptsByHash))

	// 2) 按区块高度获取整块收据
	rcptsByNum, err := fetchReceiptsByNumber(ctx, client, blockNumber)
	must(err, "block receipts by number")
	fmt.Printf("📚 Receipts(by number %s): %d\n", blockNumber.String(), len(rcptsByNum))

	// ✅ 注意：你原代码的 `receiptByHash[0] == receiptsByNum[0]` 是“指针相等”，容易误解。
	// 我们更合理的做法是比对关键字段（例如 TxHash）。
	equalFirst := false
	if len(rcptsByHash) > 0 && len(rcptsByNum) > 0 {
		equalFirst = rcptsByHash[0].TxHash == rcptsByNum[0].TxHash
	}
	fmt.Printf("🔎 First receipt equal by TxHash? %v\n", equalFirst)

	// 3) 打印第一条收据的关键信息（展示字段解释 + 友好格式）
	if len(rcptsByHash) > 0 {
		fmt.Println("\n—— Receipt (by hash) #0 ———————————————")
		printReceipt(rcptsByHash[0])
	}

	// 4) 按交易哈希查询单笔收据
	txHash := common.HexToHash("0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5")
	rcp, err := client.TransactionReceipt(ctx, txHash)
	must(err, "tx receipt by hash")
	fmt.Printf("\n🔗 TransactionReceipt(%s)\n", short(txHash.Hex(), 16))
	printReceipt(rcp)

	fmt.Println("\n✅ Done.")
}

// -------- 封装：批量获取收据 --------

func fetchReceiptsByHash(ctx context.Context, c *ethclient.Client, h common.Hash) ([]*types.Receipt, error) {
	return c.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(h, false))
}

func fetchReceiptsByNumber(ctx context.Context, c *ethclient.Client, n *big.Int) ([]*types.Receipt, error) {
	// rpc.BlockNumber 接受 int64；做一下溢出保护
	if n.BitLen() > 62 || n.Sign() < 0 || n.Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return nil, fmt.Errorf("block number out of int64 range: %s", n.String())
	}
	bn := rpc.BlockNumber(n.Int64())
	return c.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(bn))
}

// -------- 打印：收据关键信息（带解释） --------

func printReceipt(r *types.Receipt) {
	// 状态：1 成功，0 失败
	status := "FAIL"
	if r.Status == types.ReceiptStatusSuccessful {
		status = "SUCCESS"
	}
	fmt.Printf("  status:            %s\n", status)
	fmt.Printf("  txHash:            %s\n", r.TxHash.Hex())
	fmt.Printf("  txIndex:           %d\n", r.TransactionIndex)
	fmt.Printf("  blockNumber:       %d\n", r.BlockNumber.Uint64())
	fmt.Printf("  blockHash:         %s\n", r.BlockHash.Hex())

	// 合约创建交易时，ContractAddress 会是新合约地址；普通交易则为 0x0
	fmt.Printf("  contractAddress:   %s\n", r.ContractAddress.Hex())

	// 日志条数（事件）
	fmt.Printf("  logs:              %d entries\n", len(r.Logs))

	// 实际 gas 成交价（EIP-1559/Legacy 统一在这儿取）
	if r.EffectiveGasPrice != nil {
		totalFeeWei := new(big.Int).Mul(r.EffectiveGasPrice, big.NewInt(int64(r.GasUsed)))
		fmt.Printf("  gasUsed:           %d\n", r.GasUsed)
		fmt.Printf("  effectiveGasPrice: %s wei (≈ %.9f ETH)\n",
			r.EffectiveGasPrice.String(), weiToEth(r.EffectiveGasPrice))
		fmt.Printf("  totalFee:          %s wei (≈ %.9f ETH)\n",
			totalFeeWei.String(), weiToEth(totalFeeWei))
	} else {
		fmt.Printf("  gasUsed:           %d\n", r.GasUsed)
		fmt.Println("  effectiveGasPrice: <nil>") // 极少见于旧数据或特殊客户端
	}
}

// -------- 小工具 --------

func must(err error, where string) {
	if err != nil {
		log.Fatalf("❌ %s: %v", where, err)
	}
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func weiToEth(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}
	f := new(big.Float).SetInt(wei)
	eth := new(big.Float).Quo(f, big.NewFloat(1e18))
	val, _ := eth.Float64()
	return val
}
