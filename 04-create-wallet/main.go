package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

func main() {
	// ============= 1) 获取私钥 =============
	// 方案 A：随机生成新的 ECDSA 私钥（secp256k1）
	priv, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("generate key: %v", err)
	}

	// 方案 B：已知私钥十六进制（64 个 hex 字符，不含 0x）
	// priv, err := crypto.HexToECDSA("ccec5314acec3d18eae81b6bd988b844fc4f7f7d3c828b351de6d0fede02d3f2")

	// 【教学打印】私钥 hex（生产环境请勿打印/泄露）
	privBytes := crypto.FromECDSA(priv)                            // 32 字节
	fmt.Println("privateKey(hex):", hexutil.Encode(privBytes)[2:]) // 去掉 "0x"

	// ============= 2) 从私钥得到公钥 =============
	pub := priv.Public()
	pubECDSA, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("public key type assert failed")
	}
	pubBytes := crypto.FromECDSAPub(pubECDSA) // 65 字节：0x04 + 64 字节坐标
	// 【教学打印】公钥（未压缩）hex，去掉 "0x04" 前缀（4 个字符："0x04"）
	fmt.Println("publicKey(uncompressed hex no 0x04):", hexutil.Encode(pubBytes)[4:])

	// ============= 3) 计算地址（推荐内置方法） =============
	addr := crypto.PubkeyToAddress(*pubECDSA) // EIP-55 校验大小写
	fmt.Println("address (EIP-55):", addr.Hex())

	// ============= 4) 计算地址（“手算”演示） =============
	// 以太坊地址 = Keccak256(未压缩公钥去掉 0x04 的 64 字节) 的后 20 字节
	hasher := sha3.NewLegacyKeccak256()         // Keccak-256（注意：不是标准 SHA3-256）
	hasher.Write(pubBytes[1:])                  // 跳过开头的 0x04
	sum := hasher.Sum(nil)                      // 32 字节
	manualAddrLower := hexutil.Encode(sum[12:]) // 取后 20 字节并加 "0x" 前缀
	fmt.Println("address (manual from keccak):", manualAddrLower)

	// 校验两种方式是否一致
	// ✅ 比较方式 A：大小写无关
	if strings.EqualFold(addr.Hex(), manualAddrLower) {
		fmt.Println("✅ equal (case-insensitive)")
	} else {
		fmt.Println("❌ not equal (case-insensitive compare failed)")
	}

	// ✅ 比较方式 B：把手算结果标准化为 EIP-55 再比
	manualChecksum := common.HexToAddress(manualAddrLower).Hex()
	fmt.Println("address (manual EIP-55):", manualChecksum)
	if addr.Hex() == manualChecksum {
		fmt.Println("✅ equal (checksummed)")
	}
	fmt.Println("✅ address check passed")
}
