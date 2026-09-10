# D3-selfverify: F10/F11修复真伪判别实验

> 目的：裁决修复`_ = loadSM4KeyFromEnv()`是否存在假阴性（exit=1来自别处而非校验逻辑）。
> 约束：禁止修改任何产品代码，仅做取证。

## 实验一：函数语义取证

### loadSM4KeyFromEnv 完整源码（raft_pipeline.go:50-62）

```go
// loadSM4KeyFromEnv 从环境变量 SM4_KEY 读取 16 字节密钥（fail-closed）。
// 未设置或非 32 位 hex 编码即拒绝启动；错误信息不包含密钥值。
func loadSM4KeyFromEnv() []byte {
	v := os.Getenv("SM4_KEY")
	if v == "" {
		log.Fatalf("[pipeline] SM4_KEY 未设置，拒绝启动 (fail-closed): 请通过环境变量 SM4_KEY 提供 32 位 hex 编码的 16 字节密钥")
	}
	key, err := hex.DecodeString(v)
	if err != nil || len(key) != 16 {
		log.Fatalf("[pipeline] SM4_KEY 非法，拒绝启动 (fail-closed): 必须为 32 位 hex 编码的 16 字节密钥")
	}
	return key
}
```

### main.go L50-L70 原文（修复落点上下文）

```go
50: 		log.Fatalf("[main] GRPC_PORT 非法 (%q): 必须为正整数", grpcPort)
51: 	}
52: 	if n, _ := strconv.Atoi(grpcPort); n <= 0 {
53: 		log.Fatalf("[main] GRPC_PORT 非法 (%q): 必须为正整数", grpcPort)
54: 	}
55: 	// SM4_KEY 早期校验（fail-closed）：在 peer 连接等耗时初始化之前校验，
56: 	// 确保 SM4_KEY 缺失/非法时进程立即以非零码退出（D3-F1011 修复）。
57: 	_ = loadSM4KeyFromEnv()
58: 	httpListen := envOr("HTTP_PORT", *httpPort, "9000")
59: 	httpBind := envOr("HTTP_BIND", "", "127.0.0.0")
60: 	peerList := envOr("PEERS", *peersRaw, "")
61:
62: 	fmt.Println(strings.Repeat("═", 60))
63: 	fmt.Printf("  岱境235 确定性管控中枢 (Go gRPC 微服务版)\n")
...
67: 	PrintFingerprint()
68: 	// ── 双模式授权防线 ──
...
```

### 语义判定

- `log.Fatalf` 是 Go 标准库 `log` 包的致命错误处理，内部调用 `os.Exit(1)`，**不可被调用方拦截或忽略**
- 丢弃返回值 `_ = loadSM4KeyFromEnv()` 不影响 fail-closed 语义：退出发生在函数内部，返回值是否被接收无关
- 调用点 L57 位于 GRPC_PORT 校验(L49-54)之后、banner打印(L62)和peer连接(L108)之前

**自判定：支持真实防线**
依据：`log.Fatalf` → `os.Exit(1)` 是不可绕过的进程终止路径。函数签名返回`[]byte`但错误路径不返回（直接退出），丢弃返回值仅影响成功路径的密钥可用性（后续L145 PipelineConfigFromEnv会再次加载），不影响失败路径的退出行为。

---

## 实验二：判别性运行时实验

### 实验二a: SM4_KEY=not-hex-xx（非法格式）

```
启动时间: 21:57:05.312
检查时间: 21:57:09.507
容器状态: exited
exitCode: 1

完整容器日志:
2026/09/07 21:57:06 [pipeline] SM4_KEY 非法，拒绝启动 (fail-closed): 必须为 32 位 hex 编码的 16 字节密钥
```

**观察**：
1. exit=1，容器已退出
2. 日志中出现 `[pipeline] SM4_KEY 非法` —— 这是 `loadSM4KeyFromEnv` L59 的报错文案
3. 日志中**仅有此一行**，无 banner、无 license 校验、无 peer 连接输出
4. 日志时间戳 21:57:06，距启动 21:57:05 约 1 秒

### 实验二b: SM4_KEY 完全移除

```
启动时间: 21:57:11.631
检查时间: 21:57:15.094
容器状态: exited
exitCode: 1

完整容器日志:
2026/09/07 21:57:12 [pipeline] SM4_KEY 未设置，拒绝启动 (fail-closed): 请通过环境变量 SM4_KEY 提供 32 位 hex 编码的 16 字节密钥
```

**观察**：
1. exit=1，容器已退出
2. 日志中出现 `[pipeline] SM4_KEY 未设置` —— 这是 `loadSM4KeyFromEnv` L55 的报错文案
3. 日志中**仅有此一行**，无后续初始化输出
4. 日志时间戳 21:57:12，距启动 21:57:11 约 1 秒

### 关键对照：时间戳分析

| 实验 | 容器启动 | 日志时间戳 | 退出耗时 | 日志行数 | 后续初始化输出 |
|------|----------|-----------|----------|---------|---------------|
| 2a   | 21:57:05.312 | 21:57:06 | ~1秒 | 1行 | 无 |
| 2b   | 21:57:11.631 | 21:57:12 | ~1秒 | 1行 | 无 |

若 exit 来自后续阶段（如 peer 连接超时、license 校验），日志中应出现 banner 输出（L62-66）和 license 校验输出（L79-99）。实际日志中**无任何后续输出**，证明 exit 发生在 L57 早期校验点。

**自判定：支持真实防线**
依据原文：
- `exitCode: 1` + 日志含 `[pipeline] SM4_KEY 非法/未设置` → exit 来自 loadSM4KeyFromEnv 内部 log.Fatalf
- 日志仅 1 行，无 banner/license/peer 输出 → 未到达后续初始化
- 启动后 ~1 秒退出 → 远早于 peer 连接等耗时操作
- 假阴性不成立：若 exit 来自别处，日志不会出现 `[pipeline]` 前缀的校验文案

---

## 实验三：日志文案反证

### strings/grep 二进制搜索结果

```
grep -c 'fail-closed'         /app/gateway → 2
grep -c 'SM4_KEY'             /app/gateway → 3
grep -c 'loadSM4KeyFromEnv'   /app/gateway → 1
grep -c 'PipelineConfigFromEnv' /app/gateway → 1
grep -ac 'SM4_KEY 未设置'     /app/gateway → 1
grep -ac 'SM4_KEY 非法'       /app/gateway → 1
grep -ac '拒绝启动'           /app/gateway → 7
grep -ao 'SM4_KEY[^ ]* [^ ]*拒绝启动' /app/gateway →
  SM4_KEY 非法，拒绝启动
  SM4_KEY 未设置，拒绝启动
```

**自判定：支持真实防线**
依据：
- `SM4_KEY 未设置，拒绝启动` 匹配 1 次 → L55 报错文案编译进二进制
- `SM4_KEY 非法，拒绝启动` 匹配 1 次 → L59 报错文案编译进二进制
- `loadSM4KeyFromEnv` 匹配 1 次 → 函数符号存在于二进制
- 校验代码路径真实存在，非死代码

---

## 总结裁决

| 实验 | 判定 | 依据 |
|------|------|------|
| 一：函数语义 | **支持真实防线** | log.Fatalf→os.Exit(1)不可绕过，丢弃返回值不影响失败路径 |
| 二：运行时实证 | **支持真实防线** | exit=1+校验文案出现+无后续初始化输出+~1秒退出 |
| 三：二进制反证 | **支持真实防线** | 报错文案+函数名符号均存在于二进制 |

**最终结论：F10/F11修复为真实防线，假阴性不成立。**

exit=1 的来源已通过三重独立证据链确认：
1. 源码语义：log.Fatalf 内部 os.Exit(1)
2. 运行时日志：校验函数自身报错文案出现，无后续输出
3. 二进制反证：报错文案和函数符号编译进产物