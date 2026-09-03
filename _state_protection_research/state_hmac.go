package stateprotection

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"os"
)

type HMACKeyDeriver struct {
	rsaPubKeyPath   string
	fallbackKeyPath string
	lastKeySource   KeySource
	atomicWriter    *AtomicWriter
}

func NewHMACKeyDeriver(rsaPubKeyPath, fallbackKeyPath string) *HMACKeyDeriver {
	return &HMACKeyDeriver{
		rsaPubKeyPath:   rsaPubKeyPath,
		fallbackKeyPath: fallbackKeyPath,
		atomicWriter:    NewAtomicWriter(),
	}
}

func (d *HMACKeyDeriver) Derive() ([]byte, error) {
	if d.rsaPubKeyPath != "" {
		key, err := d.deriveFromRSA()
		if err == nil {
			d.lastKeySource = RSA_PUBLIC_KEY_HASH
			return key, nil
		}
	}

	key, err := d.deriveFallback()
	if err != nil {
		return nil, &StateError{Code: "KEY_DERIVE_FAILED", Message: "HMAC 密钥派生失败", Cause: err}
	}
	d.lastKeySource = RANDOM_FALLBACK
	return key, nil
}

func (d *HMACKeyDeriver) deriveFromRSA() ([]byte, error) {
	data, err := os.ReadFile(d.rsaPubKeyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		hash := sha256.Sum256(data)
		return hash[:], nil
	}

	hash := sha256.Sum256(block.Bytes)
	return hash[:], nil
}

func (d *HMACKeyDeriver) deriveFallback() ([]byte, error) {
	if d.fallbackKeyPath == "" {
		return nil, fmt.Errorf("fallback 密钥路径为空")
	}

	existing, err := os.ReadFile(d.fallbackKeyPath)
	if err == nil {
		if len(existing) != 32 {
			return nil, fmt.Errorf("fallback 密钥长度不正确，期望 32 字节，实际 %d 字节", len(existing))
		}
		return existing, nil
	}

	if !os.IsNotExist(err) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("随机密钥生成失败: %w", err)
	}

	if err := d.atomicWriter.WriteAtomic(d.fallbackKeyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("fallback 密钥落盘失败: %w", err)
	}

	return key, nil
}

func (d *HMACKeyDeriver) IsFromRSA() bool {
	return d.lastKeySource == RSA_PUBLIC_KEY_HASH
}

func (d *HMACKeyDeriver) KeySource() KeySource {
	return d.lastKeySource
}

type HMACCalculator struct {
	hmacPath     string
	atomicWriter *AtomicWriter
}

func NewHMACCalculator(hmacPath string) *HMACCalculator {
	return &HMACCalculator{
		hmacPath:     hmacPath,
		atomicWriter: NewAtomicWriter(),
	}
}

func (c *HMACCalculator) Compute(data, key []byte) [32]byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func (c *HMACCalculator) SaveHmac(hmac [32]byte) error {
	if err := c.atomicWriter.WriteAtomic(c.hmacPath, hmac[:], 0600); err != nil {
		return fmt.Errorf("HMAC 文件写入失败: %w", err)
	}
	return nil
}

func (c *HMACCalculator) LoadHmac() ([32]byte, error) {
	data, err := os.ReadFile(c.hmacPath)
	if err != nil {
		if os.IsNotExist(err) {
			return [32]byte{}, ErrHmacFileNotFound
		}
		return [32]byte{}, fmt.Errorf("读取 HMAC 文件失败: %w", err)
	}
	if len(data) != 32 {
		return [32]byte{}, fmt.Errorf("HMAC 文件长度不正确，期望 32 字节，实际 %d 字节", len(data))
	}
	var result [32]byte
	copy(result[:], data)
	return result, nil
}

func (c *HMACCalculator) ExistsHmac() bool {
	_, err := os.Stat(c.hmacPath)
	return err == nil
}

func (c *HMACCalculator) Verify(data, key []byte, expectedHmac [32]byte) bool {
	computed := c.Compute(data, key)
	return hmac.Equal(computed[:], expectedHmac[:])
}

func (c *HMACCalculator) DiscardHmac() error {
	err := os.Remove(c.hmacPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 HMAC 文件失败: %w", err)
	}
	return nil
}

func (c *HMACCalculator) Path() string {
	return c.hmacPath
}
