package main

import (
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

var licenseFieldOrder = []string{
	"LICENSE_ID", "PRODUCT", "TYPE", "ISSUED_TO",
	"ISSUED_AT", "EXPIRES_AT", "MAX_NODES", "MODULES",
	"HARDWARE_BINDING", "GRACE_PERIOD_DAYS", "SIGNATURE_ALG",
	"ISSUER", "CONTACT",
}

func main() {
	fingerprint := "f91c7eddd47c9ad7a45a0c18621c6a5f"
	privateKeyPath := os.Getenv("RSA_PRIVATE_KEY_PATH")
	outputDir := `<HOME>/.raftkv\tcx4_test\licenses_v25`

	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		fmt.Printf("读取私钥失败: %v\n", err)
		os.Exit(1)
	}
	block, _ := pem.Decode(keyData)
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		fmt.Printf("解析私钥失败: %v\n", err)
		os.Exit(1)
	}

	now := time.Now().UTC()
	expires := now.Add(365 * 24 * time.Hour)

	for i := 1; i <= 5; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		licenseID := fmt.Sprintf("DJ235-COMM-%s-%03d", now.Format("20060102"), i)

		fields := map[string]string{
			"LICENSE_ID":        licenseID,
			"PRODUCT":           "raftkv",
			"TYPE":              "COMMERCIAL",
			"ISSUED_TO":         nodeID,
			"ISSUED_AT":         now.Format("2006-01-02"),
			"EXPIRES_AT":        expires.Format("2006-01-02"),
			"MAX_NODES":         "5",
			"MODULES":           "Module01_Raft;Module02_WAL_SM4;Module03_Pipeline",
			"HARDWARE_BINDING":  fingerprint,
			"GRACE_PERIOD_DAYS": "7",
			"SIGNATURE_ALG":     "RSA-2048-SHA256",
			"ISSUER":            "raftkv",
			"CONTACT":           "support",
		}

		var signContent strings.Builder
		for _, key := range licenseFieldOrder {
			signContent.WriteString(key)
			signContent.WriteString("=")
			signContent.WriteString(fields[key])
			signContent.WriteString("\n")
		}

		hashed := sha256.Sum256([]byte(signContent.String()))
		signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
		if err != nil {
			fmt.Printf("签名失败: %v\n", err)
			os.Exit(1)
		}
		signatureB64 := base64.StdEncoding.EncodeToString(signature)

		var licenseFile strings.Builder
		licenseFile.WriteString("# ============================================\n")
		licenseFile.WriteString("#  raftkv COMMERCIAL LICENSE KEY\n")
		licenseFile.WriteString("# ============================================\n\n")

		for _, key := range licenseFieldOrder {
			licenseFile.WriteString(key)
			licenseFile.WriteString("=")
			licenseFile.WriteString(fields[key])
			licenseFile.WriteString("\n")
		}
		licenseFile.WriteString("SIGNATURE=")
		licenseFile.WriteString(signatureB64)
		licenseFile.WriteString("\n")

		outputPath := fmt.Sprintf("%s\\%s.key", outputDir, nodeID)
		if err := os.WriteFile(outputPath, []byte(licenseFile.String()), 0600); err != nil {
			fmt.Printf("写入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated: %s (fp=%s)\n", outputPath, fingerprint)
	}
}
