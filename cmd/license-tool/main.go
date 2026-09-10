package main

import (
	"bufio"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

const defaultPrivateKeyPath = `<ARCHIVE>\岱境235_RSA密钥备份\daijin235_rsa_private.pem`

var licenseFieldOrder = []string{
	"LICENSE_ID", "PRODUCT", "TYPE", "ISSUED_TO",
	"ISSUED_AT", "EXPIRES_AT", "MAX_NODES", "MODULES",
	"HARDWARE_BINDING", "GRACE_PERIOD_DAYS", "SIGNATURE_ALG",
	"ISSUER", "CONTACT",
}

func main() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  岱境235 正式商业授权密钥生成工具 (License Tool)          ║")
	fmt.Println("║  RSA-2048 / SHA256 非对称数字签名                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	privateKeyPath := defaultPrivateKeyPath
	fmt.Printf("私钥文件路径 [默认: %s]:\n> ", defaultPrivateKeyPath)
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		privateKeyPath = strings.TrimSpace(input)
	}

	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		fmt.Printf("✗ 加载私钥失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 私钥加载成功: %s\n\n", privateKeyPath)

	fmt.Println("请输入甲方硬件指纹（Hardware Fingerprint）:")
	fmt.Print("> ")
	fingerprint, _ := reader.ReadString('\n')
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		fmt.Println("✗ 硬件指纹不能为空")
		os.Exit(1)
	}
	fmt.Printf("✓ 硬件指纹: %s\n\n", fingerprint)

	fmt.Println("请输入客户名称 [默认: 商业客户]:")
	fmt.Print("> ")
	issuedTo, _ := reader.ReadString('\n')
	issuedTo = strings.TrimSpace(issuedTo)
	if issuedTo == "" {
		issuedTo = "商业客户"
	}

	fmt.Println("请输入授权天数 [默认: 365]:")
	fmt.Print("> ")
	daysStr, _ := reader.ReadString('\n')
	daysStr = strings.TrimSpace(daysStr)
	days := 365
	if daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
	}

	fmt.Println("请输入最大节点数 [默认: 5]:")
	fmt.Print("> ")
	nodesStr, _ := reader.ReadString('\n')
	nodesStr = strings.TrimSpace(nodesStr)
	maxNodes := 5
	if nodesStr != "" {
		fmt.Sscanf(nodesStr, "%d", &maxNodes)
	}

	fmt.Println("请输入输出文件路径 [默认: commercial.license.key]:")
	fmt.Print("> ")
	outputPath, _ := reader.ReadString('\n')
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		outputPath = "commercial.license.key"
	}

	now := time.Now().UTC()
	expires := now.Add(time.Duration(days) * 24 * time.Hour)
	licenseID := fmt.Sprintf("DJ235-COMM-%s-%03d", now.Format("20060102"), 1)

	fields := map[string]string{
		"LICENSE_ID":        licenseID,
		"PRODUCT":           "岱境235_微内核模块化商业组件",
		"TYPE":              "COMMERCIAL",
		"ISSUED_TO":         issuedTo,
		"ISSUED_AT":         now.Format("2006-01-02"),
		"EXPIRES_AT":        expires.Format("2006-01-02"),
		"MAX_NODES":         fmt.Sprintf("%d", maxNodes),
		"MODULES":           "Module01_Raft;Module02_WAL_SM4;Module03_Pipeline;Module04_Observability;Module05_Supervisor",
		"HARDWARE_BINDING":  fingerprint,
		"GRACE_PERIOD_DAYS": "7",
		"SIGNATURE_ALG":     "RSA-2048-SHA256",
		"ISSUER":            "岱境235 商业授权中心",
		"CONTACT":           "授权咨询请联系我方商务团队",
	}

	var signContent strings.Builder
	for _, key := range licenseFieldOrder {
		signContent.WriteString(key)
		signContent.WriteString("=")
		signContent.WriteString(fields[key])
		signContent.WriteString("\n")
	}

	contentBytes := []byte(signContent.String())
	hashed := sha256.Sum256(contentBytes)

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		fmt.Printf("✗ RSA 签名失败: %v\n", err)
		os.Exit(1)
	}
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	var licenseFile strings.Builder
	licenseFile.WriteString("# ============================================================\n")
	licenseFile.WriteString("#  岱境235 正式商业授权密钥 (COMMERCIAL LICENSE KEY)\n")
	licenseFile.WriteString("# ============================================================\n")
	licenseFile.WriteString("#  本密钥绑定硬件指纹，具备完整法律效力。\n")
	licenseFile.WriteString("#  任何篡改将导致 RSA 签名验证失败。\n")
	licenseFile.WriteString("# ============================================================\n")
	licenseFile.WriteString("\n")

	for _, key := range licenseFieldOrder {
		licenseFile.WriteString(key)
		licenseFile.WriteString("=")
		licenseFile.WriteString(fields[key])
		licenseFile.WriteString("\n")
	}

	licenseFile.WriteString("SIGNATURE=")
	licenseFile.WriteString(signatureB64)
	licenseFile.WriteString("\n")

	if err := os.WriteFile(outputPath, []byte(licenseFile.String()), 0600); err != nil {
		fmt.Printf("✗ 写入授权文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  ✓ 正式商业授权密钥生成成功                                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  输出文件:     %s\n", outputPath)
	fmt.Printf("  授权ID:       %s\n", licenseID)
	fmt.Printf("  客户名称:     %s\n", issuedTo)
	fmt.Printf("  硬件绑定:     %s\n", fingerprint)
	fmt.Printf("  授权有效期:   %s 至 %s (%d 天)\n", now.Format("2006-01-02"), expires.Format("2006-01-02"), days)
	fmt.Printf("  最大节点数:   %d\n", maxNodes)
	fmt.Printf("  签名算法:     RSA-2048 / SHA256\n")
	fmt.Printf("  签名长度:     %d bytes (Base64: %d 字符)\n", len(signature), len(signatureB64))
	fmt.Println()
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("PEM 解码失败")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	return privateKey, nil
}
