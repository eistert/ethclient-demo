package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rpcURL       = "https://ethereum-sepolia-rpc.publicnode.com"
	contractAddr = "0xbAB8279bA4FDE67A871c8E7df6E74CBAe887f118"
	privateKeyHx = "your private key" // 不要在生产中硬编码
	timeout      = 30 * time.Second
)

func main() {
	// 1) 连接节点与上下文
	client, err := ethclient.Dial(rpcURL)
	mustOK("ethclient.Dial", err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 2) 账户&链参数
	priv, err := crypto.HexToECDSA(privateKeyHx)
	mustOK("HexToECDSA", err)
	pub := priv.Public().(*ecdsa.PublicKey)
	from := crypto.PubkeyToAddress(*pub)

	nonce, err := client.PendingNonceAt(ctx, from)
	mustOK("PendingNonceAt", err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	mustOK("SuggestGasPrice", err)

	chainID, err := client.NetworkID(ctx)
	mustOK("NetworkID", err)

	// —— 打印上下文信息 ——
	to := common.HexToAddress(contractAddr)
	fmt.Println("[Contract/execute via ABI]")
	fmt.Printf("  rpc:        %s\n", rpcURL)
	fmt.Printf("  chainId:    %s\n", chainID.String())
	fmt.Printf("  from:       %s (%s)\n", from.Hex(), short(from.Hex()))
	fmt.Printf("  to:         %s (%s)\n", to.Hex(), short(to.Hex()))
	fmt.Printf("  nonce:      %d\n", nonce)
	fmt.Printf("  gasPrice:   %s Gwei\n", toGwei(gasPrice))

	// 3) 解析 ABI（直接内联 JSON，生产可读取 .abi 文件）
	const storeABI = `[{"inputs":[{"internalType":"string","name":"_version","type":"string"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"bytes32","name":"key","type":"bytes32"},{"indexed":false,"internalType":"bytes32","name":"value","type":"bytes32"}],"name":"ItemSet","type":"event"},{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"items","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"key","type":"bytes32"},{"internalType":"bytes32","name":"value","type":"bytes32"}],"name":"setItem","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"version","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}]`
	contractABI, err := abi.JSON(strings.NewReader(storeABI))
	mustOK("abi.JSON", err)

	// 4) 业务入参（bytes32）
	var key, value [32]byte
	copy(key[:], []byte("demo_save_key_use_abi"))
	copy(value[:], []byte("demo_save_value_use_abi_11111"))

	fmt.Printf("  key:        0x%s\n", hex.EncodeToString(key[:]))
	fmt.Printf("  value:      0x%s\n", hex.EncodeToString(value[:]))

	// 5) 打包 calldata（setItem(bytes32,bytes32)）
	input, err := contractABI.Pack("setItem", key, value)
	mustOK("ABI.Pack(setItem)", err)

	// 6) 构造&签名&发送交易（legacy 示例；也可改 EIP-1559）
	gasLimit := uint64(300000) // 示例值，生产建议 EstimateGas 再加 buffer
	tx := types.NewTransaction(nonce, to, big.NewInt(0), gasLimit, gasPrice, input)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), priv)
	mustOK("SignTx", err)

	err = client.SendTransaction(ctx, signedTx)
	mustOK("SendTransaction", err)
	fmt.Printf("  tx.hash:    %s\n", signedTx.Hash().Hex())
	fmt.Printf("  progress:   broadcasted, waiting to be mined...\n")

	// 7) 等待回执
	rcpt, err := waitForReceipt(ctx, client, signedTx.Hash())
	mustOK("waitForReceipt", err)
	fmt.Printf("  mined:      block=%d  status=%d  gasUsed=%d\n",
		rcpt.BlockNumber.Uint64(), rcpt.Status, rcpt.GasUsed)

	// 8) 读调用校验（items(key)）
	callData, err := contractABI.Pack("items", key)
	mustOK("ABI.Pack(items)", err)

	callMsg := ethereum.CallMsg{To: &to, Data: callData}
	raw, err := client.CallContract(ctx, callMsg, nil)
	mustOK("CallContract", err)

	var got [32]byte
	err = contractABI.UnpackIntoInterface(&got, "items", raw)
	mustOK("UnpackIntoInterface(items)", err)

	ok := (got == value)
	fmt.Printf("  verify:     Items(key) == value ? %v\n", ok)
	if !ok {
		fmt.Printf("  got:        0x%s\n", hex.EncodeToString(got[:]))
	}

	fmt.Println("[Done]")
}

// ================= 工具函数（只为更好的打印） =================

func waitForReceipt(ctx context.Context, c *ethclient.Client, h common.Hash) (*types.Receipt, error) {
	for {
		r, err := c.TransactionReceipt(ctx, h)
		if err == nil && r != nil {
			return r, nil
		}
		if err != nil && err != ethereum.NotFound {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
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
