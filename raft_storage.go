// =========================================================================
// 岱境235 确定性引擎 — SM4-CTR 落盘加密存储层
//
// 改造三（白皮书 Phase 2 ✅ [可立刻试点]）：
//   1. SM4 国密分组密码 + CTR 模式（流密码，无需填充）
//   2. 每条记录随机 IV，输出 = IV(16B) + ciphertext
//   3. 透明叠加在 WAL 之上：写入前加密，读取时解密
//
// 依赖：github.com/tjfoc/gmsm/sm4（go.mod 已含 v1.4.1）
// 实现：sm4.NewCipher → cipher.Block → cipher.NewCTR
//
// 设计原则：
//   - 不修改 raft.go / raft_wal.go，纯叠加层
//   - 每条记录独立 IV，密文相同也不暴露模式
//   - 纯 Go 零 CGO，符合国密合规要求
// =========================================================================

package main

import (
	"bytes"
	"compress/gzip"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/tjfoc/gmsm/sm4"
)

// =========================================================================
// 编译时常量
// =========================================================================

const (
	sm4KeySize   = 16 // SM4 密钥长度 128 位
	sm4BlockSize = 16 // SM4 分组大小 16 字节
)

// =========================================================================
// SM4-CTR 加密存储
// =========================================================================

// EncryptedStorage SM4-CTR 加密存储层
// 透明叠加在 WAL 之上：写入前加密，读取时解密
type EncryptedStorage struct {
	wal *WAL
	key []byte
}

// NewEncryptedStorage 创建加密存储
// key 必须为 16 字节（SM4 128 位密钥）
func NewEncryptedStorage(walPath string, key []byte) (*EncryptedStorage, error) {
	if len(key) != sm4KeySize {
		return nil, fmt.Errorf("SM4 密钥必须为 %d 字节，当前 %d 字节", sm4KeySize, len(key))
	}

	wal, err := NewWAL(walPath)
	if err != nil {
		return nil, err
	}

	return &EncryptedStorage{
		wal: wal,
		key: key,
	}, nil
}

// =========================================================================
// SM4-CTR 加解密原语
// =========================================================================

// sm4CTREncrypt SM4-CTR 加密
// 生成随机 IV，输出格式 = IV(16B) + ciphertext
func sm4CTREncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("SM4 cipher 创建失败: %w", err)
	}

	iv := make([]byte, sm4BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("IV 生成失败: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	result := make([]byte, sm4BlockSize+len(ciphertext))
	copy(result[:sm4BlockSize], iv)
	copy(result[sm4BlockSize:], ciphertext)
	return result, nil
}

// sm4CTRDecrypt SM4-CTR 解密
// 输入格式 = IV(16B) + ciphertext
func sm4CTRDecrypt(key, data []byte) ([]byte, error) {
	if len(data) < sm4BlockSize {
		return nil, fmt.Errorf("密文长度不足，至少需要 %d 字节 IV", sm4BlockSize)
	}

	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("SM4 cipher 创建失败: %w", err)
	}

	iv := data[:sm4BlockSize]
	ciphertext := data[sm4BlockSize:]

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

// =========================================================================
// 加密存储读写接口
// =========================================================================

// AppendRaftLog 加密后追加日志到 WAL
func (es *EncryptedStorage) AppendRaftLog(log RaftLog) error {
	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	encrypted, err := sm4CTREncrypt(es.key, data)
	if err != nil {
		return fmt.Errorf("SM4-CTR 加密失败: %w", err)
	}

	return es.wal.Append(walEntry{
		Index: log.Index,
		Term:  log.Term,
		Data:  encrypted,
	})
}

// ReplayAll 读取 WAL 并解密所有日志（含快照）
func (es *EncryptedStorage) ReplayAll() ([]RaftLog, error) {
	var allLogs []RaftLog

	snapshotPath := es.wal.path + ".snapshot.gz"
	if data, err := os.ReadFile(snapshotPath); err == nil {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err == nil {
			decompressed, err := io.ReadAll(gz)
			gz.Close()
			if err == nil {
				json.Unmarshal(decompressed, &allLogs)
			}
		}
	}

	entries, err := es.wal.Replay()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		decrypted, err := sm4CTRDecrypt(es.key, entry.Data)
		if err != nil {
			return nil, fmt.Errorf("SM4-CTR 解密失败 index=%d: %w", entry.Index, err)
		}

		var log RaftLog
		if err := json.Unmarshal(decrypted, &log); err != nil {
			return nil, fmt.Errorf("反序列化失败 index=%d: %w", entry.Index, err)
		}
		allLogs = append(allLogs, log)
	}
	return allLogs, nil
}

// Snapshot 创建 gzip 压缩快照并重置 WAL
func (es *EncryptedStorage) Snapshot() (int, int64, error) {
	es.wal.Flush()
	logs, err := es.ReplayAll()
	if err != nil {
		return 0, 0, fmt.Errorf("快照读取失败: %w", err)
	}

	data, err := json.Marshal(logs)
	if err != nil {
		return 0, 0, fmt.Errorf("快照序列化失败: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return 0, 0, fmt.Errorf("快照压缩失败: %w", err)
	}
	gz.Close()

	snapshotPath := es.wal.path + ".snapshot.gz"
	tmpPath := snapshotPath + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return 0, 0, fmt.Errorf("快照写入失败: %w", err)
	}
	if err := os.Rename(tmpPath, snapshotPath); err != nil {
		return 0, 0, fmt.Errorf("快照重命名失败: %w", err)
	}

	oldSize := es.wal.offset

	es.wal.Close()
	os.Remove(es.wal.path)
	wal, err := NewWAL(es.wal.path)
	if err != nil {
		return 0, 0, fmt.Errorf("WAL 重创建失败: %w", err)
	}
	es.wal = wal

	log.Printf("[storage] 快照完成: %d 条, WAL %d → 0 字节, 快照 %d 字节", len(logs), oldSize, buf.Len())
	return len(logs), oldSize, nil
}

// Close 关闭加密存储（含 WAL）
func (es *EncryptedStorage) Close() error {
	return es.wal.Close()
}

// WAL 返回底层 WAL 实例
func (es *EncryptedStorage) WAL() *WAL {
	return es.wal
}
