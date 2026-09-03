package stateprotection

import (
	"encoding/binary"
	"fmt"
	"os"
)

type StatePersistence struct {
	binPath      string
	atomicWriter *AtomicWriter
}

func NewStatePersistence(binPath string) *StatePersistence {
	return &StatePersistence{
		binPath:      binPath,
		atomicWriter: NewAtomicWriter(),
	}
}

func (sp *StatePersistence) Serialize(vr VoteRecord) []byte {
	votedForBytes := []byte(vr.VotedFor)
	buf := make([]byte, 12+len(votedForBytes))
	binary.BigEndian.PutUint64(buf[0:8], uint64(vr.Term))
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(votedForBytes)))
	copy(buf[12:], votedForBytes)
	return buf
}

func (sp *StatePersistence) Deserialize(data []byte) (VoteRecord, error) {
	if len(data) < 12 {
		return VoteRecord{}, fmt.Errorf("数据长度不足，至少需要 12 字节，当前 %d 字节", len(data))
	}

	term := int64(binary.BigEndian.Uint64(data[0:8]))
	votedForLen := binary.BigEndian.Uint32(data[8:12])

	if len(data) < 12+int(votedForLen) {
		return VoteRecord{}, fmt.Errorf("votedFor 长度不匹配，声明 %d 字节但剩余 %d 字节", votedForLen, len(data)-12)
	}

	votedFor := string(data[12 : 12+int(votedForLen)])
	return VoteRecord{Term: term, VotedFor: votedFor}, nil
}

func (sp *StatePersistence) Save(vr VoteRecord) error {
	data := sp.Serialize(vr)
	if err := sp.atomicWriter.WriteAtomic(sp.binPath, data, 0600); err != nil {
		return &StateError{Code: "PERSIST_FAILED", Message: "state.bin 写入失败", Cause: err}
	}
	return nil
}

func (sp *StatePersistence) Load() (VoteRecord, error) {
	data, err := os.ReadFile(sp.binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return VoteRecord{}, ErrStateBinNotFound
		}
		return VoteRecord{}, fmt.Errorf("读取 state.bin 失败: %w", err)
	}
	return sp.Deserialize(data)
}

func (sp *StatePersistence) Exists() bool {
	_, err := os.Stat(sp.binPath)
	return err == nil
}

func (sp *StatePersistence) Discard() error {
	err := os.Remove(sp.binPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 state.bin 失败: %w", err)
	}
	return nil
}

func (sp *StatePersistence) Path() string {
	return sp.binPath
}
