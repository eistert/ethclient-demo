package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
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
	// 1) 连接与上下文
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
	to := common.HexToAddress(contractAddr)

	chainID, err := client.NetworkID(ctx)
	mustOK("NetworkID", err)
	nonce, err := client.PendingNonceAt(ctx, from)
	mustOK("PendingNonceAt", err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	mustOK("SuggestGasPrice", err)

	// —— 打印上下文信息 ——
	fmt.Println("[Contract/execute without ABI]")
	fmt.Printf("  rpc:        %s\n", rpcURL)
	fmt.Printf("  chainId:    %s\n", chainID.String())
	fmt.Printf("  from:       %s (%s)\n", from.Hex(), short(from.Hex()))
	fmt.Printf("  to:         %s (%s)\n", to.Hex(), short(to.Hex()))
	fmt.Printf("  nonce:      %d\n", nonce)
	fmt.Printf("  gasPrice:   %s Gwei\n", toGwei(gasPrice))

	// 3) 业务入参 bytes32（Store.setItem(bytes32,bytes32)）
	var key, value [32]byte
	copy(key[:], []byte("demo_save_key_no_use_abi"))
	copy(value[:], []byte("demo_save_value_no_use_abi_11111"))
	fmt.Printf("  key:        0x%s\n", hex.EncodeToString(key[:]))
	fmt.Printf("  value:      0x%s\n", hex.EncodeToString(value[:]))

	// 4) 手动构造 calldata：selector(4) + key(32) + value(32)
	//    selector = keccak256("setItem(bytes32,bytes32)")[:4]
	setItemSelector := crypto.Keccak256([]byte("setItem(bytes32,bytes32)"))[:4]
	var input []byte
	input = append(input, setItemSelector...)
	input = append(input, key[:]...)
	input = append(input, value[:]...)

	// 5) 构造、签名并发送交易
	gasLimit := uint64(300000) // 示例值；生产建议 EstimateGas 再加 buffer
	tx := types.NewTransaction(nonce, to, big.NewInt(0), gasLimit, gasPrice, input)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), priv)
	mustOK("SignTx", err)

	err = client.SendTransaction(ctx, signedTx)
	mustOK("SendTransaction", err)
	fmt.Printf("  tx.hash:    %s\n", signedTx.Hash().Hex())
	fmt.Printf("  progress:   broadcasted, waiting to be mined...\n")

	// 6) 等待回执
	rcpt, err := waitForReceipt(ctx, client, signedTx.Hash())
	mustOK("waitForReceipt", err)
	fmt.Printf("  mined:      block=%d  status=%d  gasUsed=%d\n",
		rcpt.BlockNumber.Uint64(), rcpt.Status, rcpt.GasUsed)

	// 7) 手动构造只读查询 items(bytes32)
	itemsSelector := crypto.Keccak256([]byte("items(bytes32)"))[:4]
	var callData []byte
	callData = append(callData, itemsSelector...)
	callData = append(callData, key[:]...)

	callMsg := ethereum.CallMsg{To: &to, Data: callData}
	raw, err := client.CallContract(ctx, callMsg, nil)
	mustOK("CallContract(items)", err)

	// 8) 解析返回值：单一 bytes32，ABI 编码即 32 字节
	if len(raw) < 32 {
		mustOK("decode items", errors.New("unexpected return length < 32"))
	}
	var got [32]byte
	copy(got[:], raw[:32])

	ok := (got == value)
	fmt.Printf("  verify:     Items(key) == value ? %v\n", ok)
	if !ok {
		fmt.Printf("  got:        0x%s\n", hex.EncodeToString(got[:]))
	}

	fmt.Println("[Done]")
}

// ============== 辅助函数（仅为更好的打印） ==============

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

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return fmt.Sprintf("%s...%s", s[:6], s[len(s)-4:])
}

func toGwei(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	f := new(big.Float).SetInt(wei)
	g := new(big.Float).Quo(f, big.NewFloat(math.Pow10(9)))
	return g.Text('f', 2)
}
