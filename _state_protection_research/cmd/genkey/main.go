package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	dir := filepath.Join("..", "..", "keys")
	os.MkdirAll(dir, 0755)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("生成密钥失败: %v\n", err)
		os.Exit(1)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	})

	os.WriteFile(filepath.Join(dir, "private.pem"), privateKeyPEM, 0600)
	os.WriteFile(filepath.Join(dir, "public.pem"), publicKeyPEM, 0644)

	fmt.Println("RSA密钥对已生成:")
	fmt.Println("  keys/private.pem")
	fmt.Println("  keys/public.pem")
}
