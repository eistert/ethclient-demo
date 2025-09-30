// send_tx.go
package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	defaultRPC = "https://sepolia.infura.io/v3/<YOUR_INFURA_PROJECT_ID>"
	defaultTo  = "0x0000000000000000000000000000000000000000" // 替换为接收方
	defaultETH = "0.001"                                      // 转账金额（ETH）
	timeout    = 30 * time.Second
)

func main() {
	// 环境变量：SEPOLIA_RPC / PRIV_KEY_HEX / TO / AMOUNT_ETH
	rpcURL := getenv("SEPOLIA_RPC", defaultRPC)
	toHex := getenv("TO", defaultTo)
	amountEth := getenv("AMOUNT_ETH", defaultETH)

	if len(os.Args) < 2 && getenv("PRIV_KEY_HEX", "") == "" {
		log.Fatalf("usage: PRIV_KEY_HEX=<hex> go run send_tx.go\n(hint) export SEPOLIA_RPC/TO/AMOUNT_ETH for convenience")
	}
	// privHex := getenv("PRIV_KEY_HEX", os.Args[1]) // 也支持作为第一个参数传入

	// --- 原有代码附近，替换成下面这段 ---
	privHex := os.Getenv("PRIV_KEY_HEX")
	if privHex == "" {
		if len(os.Args) >= 2 {
			privHex = os.Args[1] // 允许用第一个位置参数传私钥
		} else {
			log.Fatalf("PRIV_KEY_HEX not set. Use: export PRIV_KEY_HEX=<hex>  or  go run send_tx.go <hex>")
		}
	}

	// 1) 连接
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := ethclient.DialContext(ctx, rpcURL)
	mustOK("ethclient.Dial", err)
	defer client.Close()

	// 2) 账户与链参数
	priv, err := crypto.HexToECDSA(privHex)
	mustOK("HexToECDSA", err)

	// priv 是 *ecdsa.PrivateKey
	pubAny := priv.Public()                 // interface{}
	pubKey, ok := pubAny.(*ecdsa.PublicKey) // 断言为 *ecdsa.PublicKey
	if !ok {
		log.Fatal("public key is not *ecdsa.PublicKey")
	}

	from := crypto.PubkeyToAddress(*pubKey)
	to := common.HexToAddress(toHex)

	chainID, err := client.NetworkID(ctx)
	mustOK("NetworkID", err)

	nonce, err := client.PendingNonceAt(ctx, from)
	mustOK("PendingNonceAt", err)

	// 3) EIP-1559 手续费（MaxPriorityFee：建议小费；MaxFee：粗略= BaseFee + 2*Tip）
	tip, err := client.SuggestGasTipCap(ctx)
	mustOK("SuggestGasTipCap", err)
	head, err := client.HeaderByNumber(ctx, nil)
	mustOK("HeaderByNumber", err)

	maxFee := new(big.Int).Add(head.BaseFee, new(big.Int).Mul(big.NewInt(2), tip))

	// 4) 转账金额（ETH → wei）
	amountWei, ok := ethToWei(amountEth)
	if !ok {
		log.Fatalf("invalid AMOUNT_ETH: %s", amountEth)
	}

	// 5) 估算 GasLimit
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:      from,
		To:        &to,
		Value:     amountWei,
		GasTipCap: tip,
		GasFeeCap: maxFee,
	})
	mustOK("EstimateGas", err)
	// 给点 buffer
	gasLimit = uint64(float64(gasLimit) * 1.15)

	// 6) 构造动态费交易并签名
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     amountWei,
		Gas:       gasLimit,
		GasTipCap: tip,
		GasFeeCap: maxFee,
		Data:      nil, // 普通转账无数据
	})
	signer := types.LatestSignerForChainID(chainID)
	signed, err := types.SignTx(tx, signer, priv)
	mustOK("SignTx", err)

	// 7) 发送交易
	err = client.SendTransaction(ctx, signed)
	mustOK("SendTransaction", err)

	// —— 打印优化 —— //
	fmt.Println("[Tx/send]")
	fmt.Printf("  rpc:         %s\n", rpcURL)
	fmt.Printf("  chainId:     %s\n", chainID.String())
	fmt.Printf("  from:        %s (%s)\n", from.Hex(), shortStr(from.Hex()))
	fmt.Printf("  to:          %s (%s)\n", to.Hex(), shortStr(to.Hex()))
	fmt.Printf("  nonce:       %d\n", nonce)
	fmt.Printf("  amount:      %s ETH\n", amountEth)
	fmt.Printf("  tip:         %s Gwei\n", toGwei(tip))
	fmt.Printf("  maxFee:      %s Gwei\n", toGwei(maxFee))
	fmt.Printf("  gasLimit:    %d\n", gasLimit)
	fmt.Printf("  tx.hash:     %s\n", signed.Hash().Hex())
	fmt.Printf("  progress:    broadcasted, waiting to be mined...\n")

	// 8) 等待上链并输出回执摘要
	rcpt := waitReceipt(ctx, client, signed.Hash())
	fmt.Printf("  mined:       block=%d  status=%d  gasUsed=%d\n",
		rcpt.BlockNumber.Uint64(), rcpt.Status, rcpt.GasUsed)
	fmt.Println("[Done]")
}

// ========== utils ==========

func waitReceipt(ctx context.Context, c *ethclient.Client, h common.Hash) *types.Receipt {
	for {
		r, err := c.TransactionReceipt(ctx, h)
		if err == nil && r != nil {
			return r
		}
		select {
		case <-ctx.Done():
			log.Fatalf("[ERR] wait receipt timeout: %v", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func ethToWei(s string) (*big.Int, bool) {
	// 支持最多 18 位小数的简易转换
	// 例如 "0.001" -> 1e15 wei
	f, ok := new(big.Float).SetString(s)
	if !ok {
		return nil, false
	}
	weiF := new(big.Float).Mul(f, big.NewFloat(math.Pow10(18)))
	wei := new(big.Int)
	weiF.Int(wei) // 向下取整
	return wei, true
}

func toGwei(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	f := new(big.Float).SetInt(wei)
	g := new(big.Float).Quo(f, big.NewFloat(math.Pow10(9)))
	return g.Text('f', 2)
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
func shortStr(s string) string {
	if len(s) <= 12 {
		return s
	}
	return fmt.Sprintf("%s...%s", s[:6], s[len(s)-4:])
}
