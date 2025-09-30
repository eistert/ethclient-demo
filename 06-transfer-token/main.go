package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"golang.org/x/crypto/sha3"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 建议把密钥改为环境变量读取；这里为演示方便先写死
const rpcURL = "https://eth-sepolia.g.alchemy.com/v2/fBR8OwccYIS5h7DcKaQ53"

func printHex(label string, b []byte) {
	fmt.Printf("%-26s %s\n", label+":", hexutil.Encode(b))
}

func printBig(label string, x *big.Int) {
	fmt.Printf("%-26s %s (dec)\n", label+":", x.String())
}

func printTitle(title string) {
	fmt.Println("==================================================")
	fmt.Println(title)
	fmt.Println("==================================================")
}

func main() {
	printTitle("STEP 1. 连接链与账户准备")
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := crypto.HexToECDSA("<你的私钥HEX>")
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	fmt.Printf("%-26s %s\n", "fromAddress:", fromAddress.Hex())

	// 用 PendingNonceAt 拿下一笔交易的 nonce
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%-26s %d\n", "nonce:", nonce)

	printTitle("STEP 2. 交易外层参数")
	// value=0：代币转账不需要转原生 ETH。
	value := big.NewInt(0) // 0 ETH
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	printBig("suggested gasPrice", gasPrice)

	// 收款人（拿到代币的人）
	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	fmt.Printf("%-26s %s\n", "toAddress:", toAddress.Hex())

	// 代币合约地址（RDT 合约本体）
	tokenAddress := common.HexToAddress("0x28b149020d2152179873ec60bed6bf7cd705775d")
	fmt.Printf("%-26s %s\n", "tokenAddress:", tokenAddress.Hex())

	printTitle("STEP 3. 手写 ABI 编码 transfer(address,uint256)")
	transferFnSignature := []byte("transfer(address,uint256)")

	// Keccak-256(函数签名) → 取前4字节
	hash := sha3.NewLegacyKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]
	printHex("methodID", methodID) // 0xa9059cbb

	// address 参数：左填充至32字节
	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	printHex("padded toAddress(32B)", paddedAddress)

	// 数量参数（假设18位小数，这里=1000个代币）
	amount := new(big.Int)
	amount.SetString("1000000000000000000000", 10) // 1000 * 10^18
	printBig("amount", amount)

	// 数量参数：左填充至32字节
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
	printHex("padded amount(32B)", paddedAmount)

	// 组装最终 data = 4 + 32 + 32 = 68 字节
	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)
	printHex("final data", data)
	fmt.Printf("%-26s %d bytes\n", "data length", len(data))

	printTitle("STEP 4. 估算 Gas（对合约调用）")
	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From: fromAddress,
		To:   &tokenAddress, // ★ 关键：估算的目标必须是“代币合约地址”
		Data: data,
		// Value: 0 // 默认即0，这里也可以显式写上
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%-26s %d\n", "estimated gasLimit", gasLimit)

	printTitle("STEP 5. 构造并签名交易")
	tx := types.NewTransaction(nonce, tokenAddress, value, gasLimit, gasPrice, data)

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%-26s %s\n", "chainID", chainID.String())

	// 拿 chainID，用 EIP-155 签名生成 signedTx。
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Fatal(err)
	}

	printTitle("STEP 6. 发送交易")
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("tx sent: %s\n", signedTx.Hash().Hex())
}
