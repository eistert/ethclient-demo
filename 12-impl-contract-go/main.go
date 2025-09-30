package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	// ⚠️ 按你的 go.mod 替换为实际路径
	store "example.com/ethclient-demo/12-impl-contract/store"
)

const (
	rpcURL       = "https://ethereum-sepolia-rpc.publicnode.com"
	contractAddr = "0xbAB8279bA4FDE67A871c8E7df6E74CBAe887f118"
	privHex      = "your private key" // 不要在生产中硬编码
	timeout      = 30 * time.Second
)

func main() {
	// 1) 连接节点
	client, err := ethclient.Dial(rpcURL)
	mustOK("ethclient.Dial", err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 2) 基本链路/账户
	chainID, err := client.NetworkID(ctx)
	mustOK("NetworkID", err)

	priv, err := crypto.HexToECDSA(privHex)
	mustOK("HexToECDSA", err)
	from := crypto.PubkeyToAddress(*priv.Public().(*ecdsa.PublicKey))

	// 3) 加载合约实例
	addr := common.HexToAddress(contractAddr)
	inst, err := store.NewStore(addr, client)
	mustOK("store.NewStore", err)

	// —— 打印上下文信息 ——
	fmt.Println("[Contract/execute]")
	fmt.Printf("  rpc:        %s\n", rpcURL)
	fmt.Printf("  chainId:    %s\n", chainID.String())
	fmt.Printf("  contract:   %s (%s)\n", addr.Hex(), short(addr.Hex()))
	fmt.Printf("  caller:     %s (%s)\n", from.Hex(), short(from.Hex()))

	// 4) 组装入参（bytes32）
	var key, value [32]byte
	copy(key[:], []byte("demo_save_key"))
	copy(value[:], []byte("demo_save_value11111"))

	fmt.Printf("  key:        0x%s\n", hex.EncodeToString(key[:]))
	fmt.Printf("  value:      0x%s\n", hex.EncodeToString(value[:]))

	// 5) 交易选项（EIP-155）
	txOpt, err := bind.NewKeyedTransactorWithChainID(priv, chainID)
	mustOK("NewKeyedTransactorWithChainID", err)
	// txOpt.GasPrice / GasFeeCap / GasTipCap / GasLimit 留空，交给节点估算更稳（也可手动设置）

	// 6) 发送交易（写操作 -> sendRawTransaction）
	tx, err := inst.SetItem(txOpt, key, value)
	mustOK("Store.SetItem", err)
	fmt.Printf("  tx.hash:    %s\n", tx.Hash().Hex())
	fmt.Printf("  progress:   broadcasted, waiting to be mined...\n")

	// 7) 等待上链并打印回执
	rcpt := waitReceipt(ctx, client, tx.Hash())
	fmt.Printf("  mined:      block=%d  status=%d  gasUsed=%d\n",
		rcpt.BlockNumber.Uint64(), rcpt.Status, rcpt.GasUsed)

	// 8) 只读查询校验（读操作 -> eth_call）
	callOpt := &bind.CallOpts{Context: ctx}
	got, err := inst.Items(callOpt, key)
	mustOK("Store.Items", err)
	ok := (got == value)
	fmt.Printf("  verify:     Items(key) == value ? %v\n", ok)
	if !ok {
		fmt.Printf("  got:        0x%s\n", hex.EncodeToString(got[:]))
	}

	fmt.Println("[Done]")
}

// ============== 打印与辅助 ==============

func waitReceipt(ctx context.Context, c *ethclient.Client, txHash common.Hash) *types.Receipt {
	for {
		r, err := c.TransactionReceipt(ctx, txHash)
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

func mustOK(tag string, err error) {
	if err != nil {
		log.Fatalf("[ERR] %s: %v", tag, err)
	}
}

func short(hex string) string {
	if len(hex) <= 12 {
		return hex
	}
	return fmt.Sprintf("%s...%s", hex[:6], hex[len(hex)-4:])
}

func toGwei(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	f := new(big.Float).SetInt(wei)
	g := new(big.Float).Quo(f, big.NewFloat(math.Pow10(9)))
	return g.Text('f', 2)
}
