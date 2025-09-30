package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types" // ← 新增这个
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	store "example.com/ethclient-demo/10-deploy-contract/store" // abigen 生成的包：--pkg=store --out=store.go
)

func main() {

	// 1) 连接节点 （示例：Sepolia）
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/fBR8OwccYIS5h7DcKaQ53")
	mustOK("ethclient.Dial", err)
	defer client.Close()

	// 2) 私钥与地址
	privateKey, err := crypto.HexToECDSA("1a6c59d835e0036630d0337caba6c68628268e4057c08d69a0726e915c7e4714")
	mustOK("HexToECDSA", err)
	pub := privateKey.Public().(*ecdsa.PublicKey)
	from := crypto.PubkeyToAddress(*pub)

	// 3) 基本链上参数
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	nonce, err := client.PendingNonceAt(ctx, from)
	mustOK("PendingNonceAt", err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	mustOK("SuggestGasPrice", err)

	chainID, err := client.NetworkID(ctx)
	mustOK("NetworkID", err)

	// 4) 构建交易授权
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	mustOK("NewKeyedTransactorWithChainID", err)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)     // 部署无需附带 ETH
	auth.GasLimit = uint64(300000) // 示例值，请按实际估算
	auth.GasPrice = gasPrice

	// —— 打印部署前信息 ——
	fmt.Println("[Deploy/abigen]")
	fmt.Printf("  from:        %s (%s)\n", from.Hex(), short(from.Hex()))
	fmt.Printf("  chainId:     %s\n", chainID.String())
	fmt.Printf("  nonce:       %d\n", nonce)
	fmt.Printf("  gasPrice:    %s Gwei\n", toGwei(gasPrice))
	fmt.Printf("  gasLimit:    %d\n", auth.GasLimit)

	// 5) 调用 abigen 部署
	input := "1.0"
	addr, tx, instance, err := store.DeployStore(auth, client, input)
	mustOK("DeployStore", err)

	// —— 打印发送结果 ——
	fmt.Printf("  tx.hash:     %s\n", tx.Hash().Hex())
	fmt.Printf("  contract?:   pending -> %s (待上链)\n", addr.Hex())

	_ = instance // 示例保持不使用

	// 6) 等待回执（简单轮询）
	receipt := waitReceipt(ctx, client, tx.Hash())
	fmt.Printf("  mined:       block=%d  status=%d  gasUsed=%d\n",
		receipt.BlockNumber.Uint64(), receipt.Status, receipt.GasUsed)
	fmt.Printf("  contract:    %s\n", receipt.ContractAddress.Hex())

	fmt.Println("[Done]")
}

// ==================== 辅助函数（只优化打印，不改变逻辑） ====================

func mustOK(tag string, err error) {
	if err != nil {
		log.Fatalf("[ERR] %s: %v", tag, err)
	}
}

func toGwei(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	f := new(big.Float).SetInt(wei)
	g := new(big.Float).Quo(f, big.NewFloat(math.Pow10(9)))
	return g.Text('f', 2)
}

func short(hex string) string {
	if len(hex) <= 12 {
		return hex
	}
	return fmt.Sprintf("%s...%s", hex[:6], hex[len(hex)-4:])
}

func waitReceipt(ctx context.Context, c *ethclient.Client, tx common.Hash) *types.Receipt {
	for {
		r, err := c.TransactionReceipt(ctx, tx)
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
