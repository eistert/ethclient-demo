package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"example.com/ethclient-demo/14-task2/counter"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	ctx := context.Background()

	rpcURL := mustGetenv("SEPOLIA_RPC")
	privHex := mustGetenv("PRIV_KEY_HEX")
	chainIDStr := os.Getenv("CHAIN_ID")
	if chainIDStr == "" {
		chainIDStr = "11155111" // Sepolia
	}
	chainID, _ := new(big.Int).SetString(chainIDStr, 10)

	// 1) 连接 Sepolia
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}
	defer client.Close()

	// 2) 加载私钥与账户
	privateKey, err := crypto.HexToECDSA(privHex)
	if err != nil {
		log.Fatalf("bad PRIV_KEY_HEX: %v", err)
	}
	publicKey := privateKey.Public()
	pubECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot cast public key")
	}
	fromAddr := crypto.PubkeyToAddress(*pubECDSA)
	fmt.Printf("Using account: %s\n", fromAddr.Hex())

	// 3) 构造交易授权 (EIP-1559)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("new transactor: %v", err)
	}
	// 让 geth 自动估算 gas；也可以手动设置
	// auth.GasFeeCap / GasTipCap / GasLimit 留空交给节点估算即可

	// 4) 部署合约（也可跳过这步，用已有地址）
	initValue := big.NewInt(42)
	contractAddr, deployTx, c, err := counter.DeployCounter(auth, client, initValue)
	if err != nil {
		log.Fatalf("deploy: %v", err)
	}
	fmt.Printf("Deployment tx: %s\n", deployTx.Hash().Hex())
	fmt.Printf("Contract address (pending): %s\n", contractAddr.Hex())

	// 等待上链
	waitMined(ctx, client, deployTx.Hash())
	fmt.Printf("Deployed at: %s\n", contractAddr.Hex())

	// 5) 读取当前值（只读调用）
	cur, err := c.Current(&bind.CallOpts{Context: ctx})
	if err != nil {
		log.Fatalf("read current(): %v", err)
	}
	fmt.Printf("current(): %s\n", cur.String())

	// 6) 调用 increment（发交易）
	tx, err := c.Increment(auth)
	if err != nil {
		log.Fatalf("increment: %v", err)
	}
	fmt.Printf("increment() tx: %s\n", tx.Hash().Hex())
	waitMined(ctx, client, tx.Hash())

	// 7) 再次读取
	cur2, err := c.Current(&bind.CallOpts{Context: ctx})
	if err != nil {
		log.Fatalf("read current() after increment: %v", err)
	}
	fmt.Printf("current() after increment: %s\n", cur2.String())
}

func waitMined(ctx context.Context, c *ethclient.Client, txHash common.Hash) {
	for {
		_, isPending, err := c.TransactionByHash(ctx, txHash)
		if err != nil {
			// 有时节点会暂时查不到，稍等重试
			time.Sleep(2 * time.Second)
			continue
		}
		if !isPending {
			break
		}
		time.Sleep(2 * time.Second)
	}
}
func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env: %s", k)
	}
	return v
}
