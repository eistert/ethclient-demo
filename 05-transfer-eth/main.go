package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const rpcURL = "https://eth-sepolia.g.alchemy.com/v2/xxxx" // ← 换成你的 RPC

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cli, err := ethclient.DialContext(ctx, rpcURL)
	must(err, "dial rpc")
	defer cli.Close()
	fmt.Println("✅ connected:", rpcURL)

	// 1) 生成“测试用”随机私钥（仅测试网）
	priv, err := crypto.GenerateKey()
	must(err, "generate key")
	fmt.Println("🔐 PRIVATE KEY (TEST ONLY):", hexutil.Encode(crypto.FromECDSA(priv))[2:])
	from := crypto.PubkeyToAddress(priv.PublicKey)
	fmt.Println("👤 from address:", from.Hex())

	// 2) 链 ID 与 nonce
	chainID, err := cli.ChainID(ctx)
	must(err, "chain id")
	nonce, err := cli.PendingNonceAt(ctx, from)
	must(err, "pending nonce")
	fmt.Println("🔢 nonce:", nonce, " chainID:", chainID)

	// 3) 转账目标与金额（改成 0.001 ETH）
	to := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d") // 换成你的收款地址
	value := new(big.Int)
	value.SetString("1000000000000000", 10) // 0.001 ETH = 1e15 wei

	// 4) EIP-1559 费用参数（无需余额即可拿到建议费率）
	tipCap, err := cli.SuggestGasTipCap(ctx)
	must(err, "suggest tip cap")
	baseLike, err := cli.SuggestGasPrice(ctx) // 兜底近似 baseFee
	must(err, "suggest gas price")
	feeCap := new(big.Int).Mul(baseLike, big.NewInt(2)) // 给个 2x 上限

	// 5) 纯转账的 gasLimit 固定 21000；避免没余额时 EstimateGas 报错
	var gasLimit uint64 = 21000

	// （可选）如果你想严格估算，也可以在有余额后再用 EstimateGas：
	_ = (&ethereum.CallMsg{
		From:      from,
		To:        &to,
		Value:     value,
		GasFeeCap: feeCap,
		GasTipCap: tipCap,
	})

	// 6) 余额检查：需要 >= value + feeCap * gasLimit
	bal, err := cli.BalanceAt(ctx, from, nil)
	must(err, "balance check")
	need := new(big.Int).Mul(feeCap, big.NewInt(int64(gasLimit)))
	need.Add(need, value)

	fmt.Printf("💳 balance = %s wei (≈ %.8f ETH)\n", bal, weiToEth(bal))
	fmt.Printf("📌 required >= value(%s) + feeCap(%s)*gas(%d) = %s wei (≈ %.8f ETH)\n",
		value, feeCap, gasLimit, need, weiToEth(need))

	if bal.Cmp(need) < 0 {
		fmt.Println("❗余额不足：请先用 Sepolia faucet 给上面的 from 地址充值，然后重跑。")
		return
	}

	// 7) 构造 EIP-1559 交易
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     value,
		Gas:       gasLimit,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
	})

	// 8) 签名并发送
	signer := types.LatestSignerForChainID(chainID)
	signed, err := types.SignTx(tx, signer, priv)
	must(err, "sign tx")

	err = cli.SendTransaction(ctx, signed)
	must(err, "send tx")
	fmt.Println("🚀 tx sent:", signed.Hash().Hex())

	// 显示最大小费上限
	maxFeeWei := new(big.Int).Mul(feeCap, big.NewInt(int64(gasLimit)))
	fmt.Printf("💰 max fee cap = %s wei (≈ %.8f ETH)\n", maxFeeWei, weiToEth(maxFeeWei))
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
	f := new(big.Float).SetInt(wei)
	eth := new(big.Float).Quo(f, big.NewFloat(1e18))
	val, _ := eth.Float64()
	return math.Round(val*1e8) / 1e8
}
