package stateprotection

import (
	"fmt"
	"os"
	"path/filepath"
)

type AtomicWriter struct{}

func NewAtomicWriter() *AtomicWriter {
	return &AtomicWriter{}
}

func (w *AtomicWriter) WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	tmpPath := path + ".tmp"

	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("fsync 临时文件失败: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename 失败: %w", err)
	}

	dirFile, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer dirFile.Close()
	dirFile.Sync()

	return nil
}
