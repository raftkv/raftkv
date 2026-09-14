package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	VERSION = "V2.1"

	C_CYAN   = "\033[36m"
	C_GREEN  = "\033[32m"
	C_BLUE   = "\033[34m"
	C_YELLOW = "\033[33m"
	C_RED    = "\033[31m"
	C_BOLD   = "\033[1m"
	C_RESET  = "\033[0m"

	MIN_RETRY_BACKOFF = 5 * time.Second
	MAX_RETRY_BACKOFF = 5 * time.Minute

	SM3_IV = 0x7380166f4914b2b9
)

var (
	useColor       bool
	currentBackoff = MIN_RETRY_BACKOFF
	logFile        *os.File
	logger         *log.Logger

	chainHashMu   sync.Mutex
	lastChainHash string

	throttleMu     sync.Mutex
	throttleActive bool

	logStorageCfg LogStorageConfig
)

type LogStorageConfig struct {
	EnableChainHash bool
	CompressDaily   bool
	WriteThrottleMB int
}

func init() {
	fi, _ := os.Stdout.Stat()
	useColor = (fi.Mode() & os.ModeCharDevice) != 0
	if os.Getenv("NO_COLOR") != "" {
		useColor = false
	}
}

func c(color, text string) string {
	if !useColor {
		return text
	}
	return color + text + C_RESET
}

func banner() {
	sep := strings.Repeat("=", 60)
	fmt.Println(c(C_CYAN, sep))
	fmt.Println(c(C_CYAN, C_BOLD+"  ⚙️  RaftKV 日志加密传输代理 "+VERSION+C_RESET))
	fmt.Println(c(C_CYAN, "  加密: AES-256-GCM | 断网: 本地缓存+指数退避"))
	fmt.Println(c(C_CYAN, "  存证: SM3哈希链防篡改 | 流控: WriteThrottle | 归档: gzip"))
	fmt.Println(c(C_CYAN, sep))
}

func sectionTitle(title string) {
	sep := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(c(C_CYAN, sep))
	fmt.Println(c(C_CYAN, "  ⚙️  "+title))
	fmt.Println(c(C_CYAN, sep))
}

func sm3Hash(data []byte) string {
	msg := make([]uint32, 8)
	msg[0] = 0x7380166f
	msg[1] = 0x4914b2b9
	msg[2] = 0x2c8e5a7e
	msg[3] = 0x6a4a6c49
	msg[4] = 0x5a3e2b1f
	msg[5] = 0x7b4d2c8a
	msg[6] = 0x3c6e1a5f
	msg[7] = 0x8b7d6a4e

	padded := sm3Pad(data)
	for i := 0; i < len(padded); i += 64 {
		block := padded[i : i+64]
		msg = sm3CF(msg, block)
	}

	var result []byte
	for _, v := range msg {
		result = append(result, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
	return hex.EncodeToString(result)
}

func sm3Pad(msg []byte) []byte {
	bitLen := uint64(len(msg)) * 8
	padded := append(msg, 0x80)
	for (len(padded)*8)%512 != 448 {
		padded = append(padded, 0x00)
	}
	lenBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		lenBytes[7-i] = byte(bitLen >> (i * 8))
	}
	padded = append(padded, lenBytes...)
	return padded
}

func sm3CF(v []uint32, block []byte) []uint32 {
	var w [68]uint32
	var w1 [64]uint32

	for i := 0; i < 16; i++ {
		w[i] = uint32(block[4*i])<<24 | uint32(block[4*i+1])<<16 | uint32(block[4*i+2])<<8 | uint32(block[4*i+3])
	}
	for i := 16; i < 68; i++ {
		w[i] = sm3P1(w[i-16]^w[i-9]^sm3Rotl(w[i-3], 15)) ^ sm3Rotl(w[i-13], 7) ^ w[i-6]
	}
	for i := 0; i < 64; i++ {
		w1[i] = w[i] ^ w[i+4]
	}

	a, b, c, d, e, f, g, h := v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7]

	for i := 0; i < 16; i++ {
		ss1 := sm3Rotl(sm3Rotl(a, 12)^e^sm3Rotl(sm3T(i), uint(i%32)), 7)
		ss2 := ss1 ^ sm3Rotl(a, 12)
		tt1 := sm3FF0(a, b, c) + d + ss2 + w1[i]
		tt2 := sm3GG0(e, f, g) + h + ss1 + w[i]
		d = c
		c = sm3Rotl(b, 9)
		b = a
		a = tt1
		h = g
		g = sm3Rotl(f, 19)
		f = e
		e = sm3P0(tt2)
	}
	for i := 16; i < 64; i++ {
		ss1 := sm3Rotl(sm3Rotl(a, 12)^e^sm3Rotl(sm3T(i), uint(i%32)), 7)
		ss2 := ss1 ^ sm3Rotl(a, 12)
		tt1 := sm3FF1(a, b, c) + d + ss2 + w1[i]
		tt2 := sm3GG1(e, f, g) + h + ss1 + w[i]
		d = c
		c = sm3Rotl(b, 9)
		b = a
		a = tt1
		h = g
		g = sm3Rotl(f, 19)
		f = e
		e = sm3P0(tt2)
	}

	return []uint32{a ^ v[0], b ^ v[1], c ^ v[2], d ^ v[3], e ^ v[4], f ^ v[5], g ^ v[6], h ^ v[7]}
}

func sm3Rotl(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}

func sm3P0(x uint32) uint32 {
	return x ^ sm3Rotl(x, 9) ^ sm3Rotl(x, 17)
}

func sm3P1(x uint32) uint32 {
	return x ^ sm3Rotl(x, 15) ^ sm3Rotl(x, 23)
}

func sm3FF0(x, y, z uint32) uint32 {
	return x ^ y ^ z
}

func sm3FF1(x, y, z uint32) uint32 {
	return (x & y) | (x & z) | (y & z)
}

func sm3GG0(x, y, z uint32) uint32 {
	return x ^ y ^ z
}

func sm3GG1(x, y, z uint32) uint32 {
	return (x & y) | (^x & z)
}

func sm3T(i int) uint32 {
	if i < 16 {
		return 0x79cc4519
	}
	return 0x7a879d8a
}

func computeChainHash(logLine string) string {
	chainHashMu.Lock()
	defer chainHashMu.Unlock()

	combined := lastChainHash + logLine
	newHash := sm3Hash([]byte(combined))
	lastChainHash = newHash
	return newHash
}

func loadLastChainHash(logPath string) {
	chainHashMu.Lock()
	defer chainHashMu.Unlock()

	f, err := os.Open(logPath)
	if err != nil {
		lastChainHash = strings.Repeat("0", 64)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lastLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "CHAIN:") {
			parts := strings.SplitN(line, "CHAIN:", 2)
			if len(parts) == 2 {
				lastLine = strings.TrimSpace(parts[1])
			}
		}
	}
	if lastLine != "" {
		lastChainHash = lastLine
	} else {
		lastChainHash = strings.Repeat("0", 64)
	}
}

func getCacheTotalSize() int64 {
	cacheDir := "./relay_cache"
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
	}
	return total
}

func isThrottleActive(cfg *Config) bool {
	if logStorageCfg.WriteThrottleMB <= 0 {
		return false
	}
	throttleMu.Lock()
	defer throttleMu.Unlock()

	cacheSizeMB := getCacheTotalSize() / (1024 * 1024)
	if cacheSizeMB >= int64(logStorageCfg.WriteThrottleMB) {
		if !throttleActive {
			throttleActive = true
			logMsg("WARN", "WriteThrottle触发！缓存达%dMB >= 阈值%dMB，降级抑制启动", cacheSizeMB, logStorageCfg.WriteThrottleMB)
		}
		return true
	}
	if throttleActive {
		throttleActive = false
		logMsg("INFO", "WriteThrottle解除，缓存降至%dMB，恢复正常写入", cacheSizeMB)
	}
	return false
}

func rotateAndCompressLog(logPath string) {
	if !logStorageCfg.CompressDaily {
		return
	}

	yesterday := time.Now().Add(-24 * time.Hour).Format("20060102")
	rotatedName := fmt.Sprintf("raftkv_%s.log", yesterday)
	rotatedPath := filepath.Join(filepath.Dir(logPath), rotatedName)

	if _, err := os.Stat(logPath); err != nil {
		return
	}

	if _, err := os.Stat(rotatedPath); err == nil {
		return
	}

	src, err := os.Open(logPath)
	if err != nil {
		logMsg("ERROR", "日志轮转: 打开原文件失败: %v", err)
		return
	}
	defer src.Close()

	fi, _ := src.Stat()
	if fi.Size() == 0 {
		return
	}

	dst, err := os.Create(rotatedPath)
	if err != nil {
		logMsg("ERROR", "日志轮转: 创建归档文件失败: %v", err)
		return
	}

	_, err = io.Copy(dst, src)
	dst.Close()
	if err != nil {
		logMsg("ERROR", "日志轮转: 复制内容失败: %v", err)
		os.Remove(rotatedPath)
		return
	}

	gzPath := rotatedPath + ".gz"
	gzFile, err := os.Create(gzPath)
	if err != nil {
		logMsg("ERROR", "日志压缩: 创建.gz文件失败: %v", err)
		return
	}

	rotatedSrc, err := os.Open(rotatedPath)
	if err != nil {
		gzFile.Close()
		os.Remove(gzPath)
		return
	}

	gzWriter := gzip.NewWriter(gzFile)
	_, err = io.Copy(gzWriter, rotatedSrc)
	gzWriter.Close()
	gzFile.Close()
	rotatedSrc.Close()

	if err != nil {
		logMsg("ERROR", "日志压缩: gzip写入失败: %v", err)
		os.Remove(gzPath)
		return
	}

	os.Remove(rotatedPath)

	truncFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		truncFile.Close()
	}

	loadLastChainHash(logPath)

	logMsg("INFO", "日志归档完成: %s → %s (%.1fKB→%.1fKB)", rotatedName, filepath.Base(gzPath),
		float64(fi.Size())/1024, func() float64 {
			gzInfo, _ := os.Stat(gzPath)
			if gzInfo != nil {
				return float64(gzInfo.Size()) / 1024
			}
			return 0
		}())
}

func dailyRotateScheduler(logPath string) {
	for {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 5, 0, now.Location())
		waitDur := nextMidnight.Sub(now)
		time.Sleep(waitDur)

		logMsg("INFO", "⏰ 零点轮转触发，开始归档前一天日志")
		rotateAndCompressLog(logPath)
	}
}

func logMsg(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006/01/02 15:04:05")

	var color string
	var icon string
	switch level {
	case "INFO":
		color = C_GREEN
		icon = "✅"
	case "WARN":
		color = C_YELLOW
		icon = "⚠️"
	case "ERROR":
		color = C_RED
		icon = "❌"
	case "CACHE":
		color = C_BLUE
		icon = "📁"
	case "BACKOFF":
		color = C_YELLOW
		icon = "⏳"
	case "RETRY":
		color = C_GREEN
		icon = "🔄"
	case "RETRY-FAIL":
		color = C_RED
		icon = "❌"
	case "THROTTLE":
		color = C_RED
		icon = "🚨"
	default:
		color = ""
		icon = ""
	}

	consoleLine := fmt.Sprintf("%s [%s] %s %s", timestamp, level, icon, msg)
	fmt.Println(c(color, consoleLine))

	if logger != nil {
		fileLine := fmt.Sprintf("[%s] %s", level, msg)
		if logStorageCfg.EnableChainHash {
			chainHash := computeChainHash(fileLine)
			fileLine = fileLine + " CHAIN:" + chainHash
		}
		logger.Println(fileLine)
	}
}

func progressBar(current, total int, width int) string {
	if total == 0 {
		return fmt.Sprintf("[%s] 0/0", strings.Repeat("-", width))
	}
	filled := width * current / total
	bar := strings.Repeat("=", filled)
	if filled < width {
		bar += ">"
		bar += strings.Repeat("-", width-filled-1)
	}
	return fmt.Sprintf("[%s] %d/%d", bar, current, total)
}

type Config struct {
	Mode            string
	Target          string
	EncryptKey      string
	Interval        time.Duration
	MaxLocalCacheMB int
	PackThresholdMB int
	WatchDir        string
	S3AccessKey     string
	S3SecretKey     string
	S3Bucket        string
	S3Region        string
	HBEnable        bool
	HBUrl           string
	HBInterval      time.Duration
}

func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "h") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "h"))
		return time.Duration(v) * time.Hour
	}
	if strings.HasSuffix(s, "m") && !strings.HasSuffix(s, "min") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "m"))
		return time.Duration(v) * time.Minute
	}
	if strings.HasSuffix(s, "s") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "s"))
		return time.Duration(v) * time.Second
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func parseSizeMB(s string) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if strings.HasSuffix(s, "MB") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "MB"))
		return v
	}
	if strings.HasSuffix(s, "GB") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "GB"))
		return v * 1024
	}
	v, _ := strconv.Atoi(s)
	return v
}

func loadConfig(path string) *Config {
	cfg := &Config{
		Mode:            "File",
		Target:          "./relay_output/",
		EncryptKey:      "raftkv_relay_2026_secure_key",
		Interval:        30 * time.Second,
		MaxLocalCacheMB: 20,
		PackThresholdMB: 5,
		WatchDir:        "..",
		HBEnable:        false,
		HBUrl:           "",
		HBInterval:      300 * time.Second,
	}

	logStorageCfg = LogStorageConfig{
		EnableChainHash: true,
		CompressDaily:   true,
		WriteThrottleMB: 10,
	}

	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	currentSection := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")

		if currentSection == "Output" {
			switch key {
			case "Mode":
				cfg.Mode = val
			case "Target":
				cfg.Target = val
			case "EncryptKey":
				cfg.EncryptKey = val
			case "Interval":
				cfg.Interval = parseDuration(val)
			case "MaxLocalCacheSize":
				cfg.MaxLocalCacheMB = parseSizeMB(val)
			case "PackThreshold":
				cfg.PackThresholdMB = parseSizeMB(val)
			case "WatchDir":
				cfg.WatchDir = val
			}
		} else if currentSection == "S3" {
			switch key {
			case "AccessKey":
				cfg.S3AccessKey = val
			case "SecretKey":
				cfg.S3SecretKey = val
			case "Bucket":
				cfg.S3Bucket = val
			case "Region":
				cfg.S3Region = val
			}
		} else if currentSection == "Heartbeat" {
			switch key {
			case "Enable":
				cfg.HBEnable = strings.ToLower(val) == "true"
			case "Url":
				cfg.HBUrl = val
			case "Interval":
				iv, _ := strconv.Atoi(val)
				if iv > 0 {
					cfg.HBInterval = time.Duration(iv) * time.Second
				}
			}
		} else if currentSection == "LogStorage" {
			switch key {
			case "EnableChainHash":
				logStorageCfg.EnableChainHash = strings.ToLower(val) == "true"
			case "CompressDaily":
				logStorageCfg.CompressDaily = strings.ToLower(val) == "true"
			case "WriteThrottle":
				logStorageCfg.WriteThrottleMB = parseSizeMB(val)
			}
		}
	}
	return cfg
}

func aesGCMEncrypt(plaintext []byte, key string) ([]byte, error) {
	keyBytes := deriveKey(key)
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func deriveKey(key string) []byte {
	hexStr := fmt.Sprintf("%x", key)
	for len(hexStr) < 64 {
		hexStr += hexStr
	}
	b, _ := hex.DecodeString(hexStr[:64])
	return b
}

func collectFiles(dir string) ([]string, error) {
	var files []string
	patterns := []string{"*.dat", "*.json", "*.log", "*.csv", "*.txt"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, pat := range patterns {
			matched, _ := filepath.Match(pat, name)
			if matched {
				files = append(files, filepath.Join(dir, name))
				break
			}
		}
	}
	return files, nil
}

func packAndEncrypt(files []string, key string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("DAIJIN235_PACK_V1\n")
	for _, f := range files {
		base := filepath.Base(f)
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		buf.WriteString(fmt.Sprintf("FILE:%s:SIZE:%d\n", base, len(data)))
		buf.Write(data)
		buf.WriteByte('\n')
	}
	return aesGCMEncrypt(buf.Bytes(), key)
}

func transmitFile(data []byte, filename string, cfg *Config) error {
	switch cfg.Mode {
	case "File":
		targetDir := cfg.Target
		os.MkdirAll(targetDir, 0755)
		targetPath := filepath.Join(targetDir, filename)
		return os.WriteFile(targetPath, data, 0644)

	case "USB":
		drives := []string{"D:\\", "E:\\", "F:\\", "G:\\"}
		for _, d := range drives {
			marker := filepath.Join(d, ".raftkv_usb")
			if _, err := os.Stat(marker); err == nil {
				targetDir := filepath.Join(d, "raftkv_relay")
				os.MkdirAll(targetDir, 0755)
				return os.WriteFile(filepath.Join(targetDir, filename), data, 0644)
			}
		}
		return fmt.Errorf("未检测到RaftKV标记USB设备")

	case "HTTP":
		tmpFile := filepath.Join(os.TempDir(), filename)
		os.WriteFile(tmpFile, data, 0644)
		defer os.Remove(tmpFile)
		return fmt.Errorf("HTTP上传需要curl，当前为模拟模式")

	case "S3":
		return fmt.Errorf("S3上传需要SDK，当前为模拟模式")

	default:
		return fmt.Errorf("未知传输模式: %s", cfg.Mode)
	}
}

func cacheFile(data []byte, filename string) error {
	cacheDir := "./relay_cache"
	os.MkdirAll(cacheDir, 0755)
	return os.WriteFile(filepath.Join(cacheDir, filename), data, 0644)
}

func getCachedFiles() []string {
	cacheDir := "./relay_cache"
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".enc") {
			files = append(files, filepath.Join(cacheDir, e.Name()))
		}
	}
	return files
}

func cleanOldCache(maxMB int) {
	cacheDir := "./relay_cache"
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}

	var totalSize int64
	type fileEntry struct {
		path    string
		size    int64
		modTime time.Time
	}
	var fileList []fileEntry

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
		fileList = append(fileList, fileEntry{
			path:    filepath.Join(cacheDir, e.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	maxBytes := int64(maxMB) * 1024 * 1024
	if totalSize <= maxBytes {
		return
	}

	for i := range fileList {
		for j := i + 1; j < len(fileList); j++ {
			if fileList[i].modTime.After(fileList[j].modTime) {
				fileList[i], fileList[j] = fileList[j], fileList[i]
			}
		}
	}

	for _, f := range fileList {
		if totalSize <= maxBytes {
			break
		}
		os.Remove(f.path)
		totalSize -= f.size
		logMsg("WARN", "缓存超限，已清理旧文件: %s", filepath.Base(f.path))
	}
}

func retryCachedFiles(cfg *Config) {
	cached := getCachedFiles()
	if len(cached) == 0 {
		return
	}

	logMsg("RETRY", "发现 %d 个缓存文件，尝试重传", len(cached))
	fmt.Println("  " + progressBar(0, len(cached), 30))

	allSuccess := true
	for i, f := range cached {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		filename := filepath.Base(f)
		err = transmitFile(data, filename, cfg)
		fmt.Println("  " + progressBar(i+1, len(cached), 30))
		if err != nil {
			allSuccess = false
		} else {
			os.Remove(f)
			logMsg("INFO", "缓存重传成功: %s", filename)
		}
	}

	if !allSuccess {
		logMsg("RETRY-FAIL", "部分缓存文件重传失败，将按退避策略重试")
		currentBackoff = time.Duration(math.Min(float64(currentBackoff*2), float64(MAX_RETRY_BACKOFF)))
		logMsg("BACKOFF", "重传仍有失败，退避递增至 %s", formatDuration(currentBackoff))
	} else {
		currentBackoff = MIN_RETRY_BACKOFF
		logMsg("INFO", "所有缓存文件重传成功，退避重置为 %s", formatDuration(MIN_RETRY_BACKOFF))
	}
}

func formatDuration(d time.Duration) string {
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func heartbeatLoop(url string, interval time.Duration) {
	type HeartbeatPayload struct {
		DeviceID string `json:"device_id"`
		Status   string `json:"status"`
		Ts       string `json:"timestamp"`
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		payload := HeartbeatPayload{
			DeviceID: "raftkv",
			Status:   "alive",
			Ts:       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
}

func setupLogging() {
	var err error
	logPath := "relay_agent.log"
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(c(C_RED, "❌ 无法创建日志文件: "+err.Error()))
		return
	}
	logger = log.New(logFile, "", log.Ldate|log.Ltime)

	if logStorageCfg.EnableChainHash {
		loadLastChainHash(logPath)
	}
}

func main() {
	configPath := "config.toml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg := loadConfig(configPath)

	setupLogging()
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	banner()

	if logStorageCfg.CompressDaily {
		go dailyRotateScheduler("relay_agent.log")
	}

	if cfg.HBEnable && cfg.HBUrl != "" {
		go heartbeatLoop(cfg.HBUrl, cfg.HBInterval)
	}

	sectionTitle("启动参数")
	fmt.Println(c(C_GREEN, fmt.Sprintf("  ✅ 模式: %s | 间隔: %s | 监控: %s", cfg.Mode, formatDuration(cfg.Interval), cfg.WatchDir)))
	fmt.Println(c(C_GREEN, fmt.Sprintf("  ✅ 加密: AES-256-GCM | 目标: %s", cfg.Target)))
	fmt.Println(c(C_YELLOW, fmt.Sprintf("  ⏳ 退避策略: 5s → 10s → 20s → 40s → ... → 封顶5min")))
	if cfg.HBEnable && cfg.HBUrl != "" {
		fmt.Println(c(C_GREEN, fmt.Sprintf("  ✅ 心跳: 已启用 | 间隔: %s | 目标: %s", formatDuration(cfg.HBInterval), cfg.HBUrl)))
	} else {
		fmt.Println(c(C_YELLOW, "  ⏳ 心跳: 未启用"))
	}
	fmt.Println(c(C_GREEN, fmt.Sprintf("  ✅ 哈希链防篡改: %v | 每日压缩归档: %v | 流控阈值: %dMB",
		logStorageCfg.EnableChainHash, logStorageCfg.CompressDaily, logStorageCfg.WriteThrottleMB)))

	watchDir := cfg.WatchDir
	os.MkdirAll("./relay_cache", 0755)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	lastRetryTime := time.Now()

	for {
		if isThrottleActive(cfg) {
			cleanOldCache(cfg.MaxLocalCacheMB)
			cleanOldCache(logStorageCfg.WriteThrottleMB)
		}

		files, err := collectFiles(watchDir)
		if err != nil {
			logMsg("ERROR", "扫描目录失败: %v", err)
			<-ticker.C
			continue
		}

		if len(files) > 0 {
			if isThrottleActive(cfg) {
				logMsg("THROTTLE", "流控降级中，仅保留最新%d个文件丢弃旧数据", minInt(len(files), 3))
				if len(files) > 3 {
					for i := 0; i < len(files)-3; i++ {
						os.Remove(files[i])
					}
					files = files[len(files)-3:]
				}
			}

			logMsg("INFO", "检测到 %d 个文件，开始打包加密", len(files))

			encrypted, err := packAndEncrypt(files, cfg.EncryptKey)
			if err != nil {
				logMsg("ERROR", "加密失败: %v", err)
				<-ticker.C
				continue
			}

			timestamp := time.Now().Format("20060102_150405")
			encFilename := fmt.Sprintf("raftkv_relay_%s.enc", timestamp)

			err = transmitFile(encrypted, encFilename, cfg)
			if err != nil {
				logMsg("WARN", "%s上传失败: %v, 转入缓存", cfg.Mode, err)
				cacheErr := cacheFile(encrypted, encFilename)
				if cacheErr != nil {
					logMsg("ERROR", "缓存写入失败: %v", cacheErr)
				} else {
					logMsg("CACHE", "已缓存: %s", encFilename)
				}
			} else {
				logMsg("INFO", "传输成功: %s", encFilename)
				currentBackoff = MIN_RETRY_BACKOFF
			}
		}

		cleanOldCache(cfg.MaxLocalCacheMB)

		if time.Since(lastRetryTime) >= currentBackoff {
			cached := getCachedFiles()
			if len(cached) > 0 {
				logMsg("BACKOFF", "退避等待 %s 到期，开始重试缓存文件", formatDuration(currentBackoff))
				retryCachedFiles(cfg)
			}
			lastRetryTime = time.Now()
		}

		<-ticker.C
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
