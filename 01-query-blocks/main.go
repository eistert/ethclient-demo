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

const rpcURL = "https://eth-sepolia.g.alchemy.com/v2/xxx"

// 运行后会连接节点、拿到最新区块头，再把该高度的完整区块取出来，最后打印交易数量等关键信息。

func main() {
	// 给所有 RPC 调用一个统一超时，避免网络卡住
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1) 连接到 Alchemy 的 Sepolia 节点
	cli, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}

	defer cli.Close()
	fmt.Println("✅ Connected to:", rpcURL)

	// 2) 查询“最新区块头”（传 nil 表示 latest）
	head, err := cli.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("latest header: %v", err)
	}
	fmt.Printf("📦 Latest Header => number=%v hash=%s time=%v baseFeeWei=%v\n",
		head.Number, head.Hash(), time.Unix(int64(head.Time), 0), head.BaseFee)

	// 3) 用最新高度查询“完整区块”
	block, err := cli.BlockByNumber(ctx, head.Number)
	if err != nil {
		log.Fatalf("block by number: %v", err)
	}
	fmt.Printf("📦 Block #%v => hash=%s time=%v gasUsed=%d txs=%d\n",
		block.Number(), block.Hash(), time.Unix(int64(block.Time()), 0), block.GasUsed(), len(block.Transactions()))

	// 4) 如果你想按哈希再取一次整块，也可以演示一下（这里直接用刚拿到的哈希）
	blockByHash, err := cli.BlockByHash(ctx, block.Hash())
	if err != nil {
		log.Fatalf("block by hash: %v", err)
	}
	fmt.Printf("🔎 Block (ByHash) #%v => txs=%d\n", blockByHash.Number(), len(blockByHash.Transactions()))

	// 5) 只想拿交易数量，不想拿整块时，可直接用 TransactionCount（传区块哈希）
	txCount, err := cli.TransactionCount(ctx, block.Hash())
	if err != nil {
		log.Fatalf("tx count: %v", err)
	}
	fmt.Printf("🧾 TxCount of #%v => %d\n", block.Number(), txCount)

	// 6) 示例：如何查询“指定高度”的区块（把 n 改成你感兴趣的高度）
	n := big.NewInt(5_671_744) // Sepolia 示例高度；可随意修改
	b2, err := cli.BlockByNumber(ctx, n)
	if err != nil {
		log.Fatalf("block by number(%v): %v", n, err)
	}
	fmt.Printf("📚 Block #%v => hash=%s txs=%d\n", b2.Number(), b2.Hash(), len(b2.Transactions()))

	// 7) 也演示一下：按区块哈希再拿交易数量
	cnt2, err := cli.TransactionCount(ctx, common.HexToHash(b2.Hash().Hex()))
	if err != nil {
		log.Fatalf("tx count(by hash): %v", err)
	}
	fmt.Printf("🧮 TxCount of #%v => %d\n", b2.Number(), cnt2)

	fmt.Println("✅ Done.")
}
