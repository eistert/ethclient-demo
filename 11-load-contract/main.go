package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	// ⚠️ 将下面这行替换为你生成代码的实际模块路径
	// 例如：store "example.com/ethclient-demo/10-deploy-contract/store"
	store "example.com/ethclient-demo/11-load-contract/store" // abigen 生成的包：--pkg=store --out=store.go
)

const (
	rpcURL       = "https://eth-sepolia.g.alchemy.com/v2/xxxx"  // 你的节点（Ganache/本地/测试网RPC）
	contractAddr = "0xbAB8279bA4FDE67A871c8E7df6E74CBAe887f118" // 已部署的 Store 合约地址
	timeout      = 10 * time.Second
)

func main() {
	// 1) 连接节点
	client, err := ethclient.Dial(rpcURL)
	mustOK("ethclient.Dial", err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 2) 打印基础信息
	chainID, err := client.NetworkID(ctx)
	mustOK("NetworkID", err)

	addr := common.HexToAddress(contractAddr)
	fmt.Println("[Load/abigen]")
	fmt.Printf("  rpc:       %s\n", rpcURL)
	fmt.Printf("  chainId:   %s\n", chainID.String())
	fmt.Printf("  contract:  %s (%s)\n", addr.Hex(), short(addr.Hex()))

	// 3) 加载合约实例
	inst, err := store.NewStore(addr, client)
	mustOK("store.NewStore", err)
	fmt.Println("  status:    contract instance loaded")

	// 4) 只读调用示例：读取公开变量 version
	version, err := inst.Version(&bind.CallOpts{Context: ctx})
	mustOK("Store.Version()", err)
	fmt.Printf("  version:   %q\n", version)

	fmt.Println("[Done]")
}

// ================= 工具函数（打印优化） =================

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
