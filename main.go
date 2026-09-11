// =========================================================================
// 岱境235 确定性引擎 — 主入口
//
// 启动方式（三选一）：
//   1. 命令行参数:
//      go run . -id node-1 -port 9500 -peers node-2=localhost:9501,node-3=localhost:9502
//      NODE_ID=node-1 GRPC_PORT=9500 HTTP_PORT=9001 \
//        PEERS=node-2=node-2:9501,node-3=node-3:9502 ./gateway
//   3. 混合模式: 命令行参数优先，未指定时回退到环境变量
// =========================================================================

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	Version   = "v2.5.1"
	BuildTime = "2026-09-02"
	GitCommit = "v251-sm3-integrity"
)

func main() {
	id := flag.String("id", "", "节点唯一 ID (环境变量: NODE_ID)")
	port := flag.String("port", "", "gRPC 服务端口 (环境变量: GRPC_PORT)")
	httpPort := flag.String("http", "", "HTTP API 端口 (环境变量: HTTP_PORT)")
	peersRaw := flag.String("peers", "", "Peer 列表 (环境变量: PEERS)")
	flag.Parse()

	// 环境变量回退：命令行参数为空时从环境变量读取
	nodeID := envOr("NODE_ID", *id, "node-1")
	grpcPort := envOr("GRPC_PORT", *port, "9500")
	if _, err := strconv.Atoi(grpcPort); err != nil {
		log.Fatalf("[main] GRPC_PORT 非法 (%q): 必须为正整数", grpcPort)
	}
	if n, _ := strconv.Atoi(grpcPort); n <= 0 {
		log.Fatalf("[main] GRPC_PORT 非法 (%q): 必须为正整数", grpcPort)
	}
	// SM4_KEY 早期校验（fail-closed）：在 peer 连接等耗时初始化之前校验，
	// 确保 SM4_KEY 缺失/非法时进程立即以非零码退出（D3-F1011 修复）。
	_ = loadSM4KeyFromEnv()
	httpListen := envOr("HTTP_PORT", *httpPort, "9000")
	httpBind := envOr("HTTP_BIND", "", "127.0.0.1")
	peerList := envOr("PEERS", *peersRaw, "")

	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("  岱境235 确定性管控中枢 (Go gRPC 微服务版)\n")
	fmt.Printf("  双轨三总台五级联动分布式管控系统\n")
	fmt.Printf("  版本: %s  构建: %s  提交: %s\n", Version, BuildTime, GitCommit)
	fmt.Println(strings.Repeat("═", 60))
	PrintFingerprint()
	// ── 双模式授权防线（2026-09-01 姜总裁决二）──────────────────────────────
	// LICENSE_FAIL_MODE=closed（出厂默认）→ 授权失败：log.Fatal 拒绝启动
	// LICENSE_FAIL_MODE=open             → 授权失败：降级为只读模式继续运行
	// open 模式严禁在未与甲方签署《授权到期降级只读补充条款》前开启，
	// 详见《双模式授权配置说明.md》第四节"开启前置条件与法务红线"。
	failMode := LicenseFailMode()
	if raw := LicenseFailModeRaw(); raw == "" {
		fmt.Printf("[授权] 失败处理模式: %s（环境变量 %s 未设置，回落出厂默认 %s）\n",
			failMode, LicenseFailModeEnvKey, DefaultLicenseFailMode)
	} else {
		fmt.Printf("[授权] 失败处理模式: %s（环境变量 %s=%q）\n",
			failMode, LicenseFailModeEnvKey, raw)
	}

	if err := VerifyLicense(); err != nil {
		if failMode == LicenseFailModeOpen {
			fmt.Fprintf(os.Stderr, "\n╔════════════════════════════════════════════════════╗\n")
			fmt.Fprintf(os.Stderr, "║  授权校验失败（Fail-Open 降级为只读模式运行）      ║\n")
			fmt.Fprintf(os.Stderr, "╚════════════════════════════════════════════════════╝\n")
			fmt.Fprintf(os.Stderr, "%v\n", err)
			fmt.Fprintf(os.Stderr, "\n[授权] ⚠ 当前为 open 模式：系统将在无有效授权下以只读模式继续运行。\n")
			fmt.Fprintf(os.Stderr, "[授权] ⚠ 法务提示：open 模式须以《授权到期降级只读补充条款》已签署生效为前提。\n")
			fmt.Fprintf(os.Stderr, "[授权] ⚠ 若该补充条款尚未生效，请立即将 %s 改回 closed 并重启。\n", LicenseFailModeEnvKey)
			SetDegradedMode(fmt.Sprintf("授权校验失败(Fail-Open)：%v", err))
			fmt.Fprintf(os.Stderr, "[授权] 已置位全局降级只读标志：写入请求与成员变更将被拒绝，选举与心跳保持正常。\n")
		} else {
			fmt.Fprintf(os.Stderr, "\n╔══════════════════════════════════════╗\n")
			fmt.Fprintf(os.Stderr, "║  授权校验失败（Fail-Closed 拒绝启动）║\n")
			fmt.Fprintf(os.Stderr, "╚══════════════════════════════════════╝\n")
			fmt.Fprintf(os.Stderr, "%v\n", err)
			log.Fatalf("[授权] 致命错误：授权校验失败，进程终止（Fail-Closed）")
		}
	} else {
		fmt.Println("[授权] 硬件环境验证通过")
	}

	peerAddrs := parsePeerAddrs(peerList)
	fmt.Printf("[启动] 节点 ID: %s, gRPC: %s, HTTP: %s\n", nodeID, grpcPort, httpListen)
	fmt.Printf("[启动] Peer 列表 (%d 个):\n", len(peerAddrs))
	for pid, addr := range peerAddrs {
		fmt.Printf("  - %s → %s\n", pid, addr)
	}

	peerMgr := NewPeerClientManager(peerAddrs)
	peerClients := peerMgr.ConnectAll()
	defer peerMgr.CloseAll()

	fmt.Printf("[启动] 已连接 %d/%d 个 peer\n", len(peerClients), len(peerAddrs))

	node := NewRaftNode(nodeID, peerAddrs, peerClients, nil)
	node.logger = &stdLogger{prefix: fmt.Sprintf("[raft/%s]", nodeID)}
	node.config.logger = node.logger // V2.3: 同步 logger 到集群配置

	// R-04修复A: 设置重连回调 + 启动后台重连循环
	// DNS别名丢失后gRPC "produced zero addresses"，需定期重建连接强制DNS重解析
	peerMgr.SetOnReconnect(node.UpdatePeerClient)
	peerMgr.StartReconnectLoop()
	defer peerMgr.StopReconnectLoop()

	// ── 国密 SM3 防篡改链：标准自检与装配（V2.5.1 真实启用）────────────────
	// 算法本体：github.com/tjfoc/gmsm v1.4.1（清华大学开源国密库，非本项目自研）
	// 标准依据：GB/T 32905-2016《信息安全技术 SM3 密码杂凑算法》
	// 自研部分：链式防篡改集成层 + Raft 共识核心
	sm3OK, sm3Lines := SM3SelfTest()
	if sm3OK {
		fmt.Println("[国密] SM3 标准测试向量自检通过 (GB/T 32905-2016)")
	} else {
		fmt.Println("[国密] ✗ SM3 标准测试向量自检失败，防篡改能力不可用")
	}
	for _, l := range sm3Lines {
		fmt.Println("[国密]", l)
	}
	if !sm3IntegrityEnabled {
		fmt.Printf("[国密] ⚠ 防篡改链已被环境变量 %s 关闭，生产环境严禁关闭\n", SM3IntegrityEnvKey)
	}

	// 装配统一状态机与双适配层，注入国密标准 SM3 实现
	unifiedSM, _, _ := InitAdapters(node, NewStandardSM3())
	globalUnifiedSM = unifiedSM
	fmt.Printf("[国密] 统一状态机已装配 (引擎=%T, 链头=%x)\n",
		unifiedSM.SM3Hasher, unifiedSM.LastSM3Hash[:8])

	// 初始化 Raft 日志处理管线 (WAL 加密持久化 + TiDB/MySQL 异步落盘)
	pipelineCfg := PipelineConfigFromEnv()
	pipeline, pipelineErr := NewRaftPipeline(pipelineCfg)
	if pipelineErr != nil {
		log.Printf("[启动] ⚠ 管线初始化失败（降级运行）: %v", pipelineErr)
	} else {
		node.SetOnCommit(pipeline.OnCommit)
		pipeline.SetOnSnapshotCompact(node.CompactLogs)
		scheduler := NewSnapshotScheduler(pipeline.Storage(), node.CompactLogs, node, log.New(os.Stderr, "[snapshot-sched] ", log.LstdFlags))
		scheduler.Start()
		pipeline.SetScheduler(scheduler)
		sinkOn := pipelineCfg.EnableSink && pipelineCfg.SinkConfig.Enable
		fmt.Printf("[启动] Raft 管线已接入 (WAL=%v, Sink=%v, 异步快照=启用)\n", pipelineCfg.EnableWAL, sinkOn)
		defer scheduler.Stop()
		defer pipeline.Close()
	}

	// TCX-Ⅳ: 强制WAL回放（即使管线初始化失败也执行）
	var replayStats WALReplayStats
	if pipeline != nil {
		replayedLogs, stats, replayErr := pipeline.ReplayWALWithStats()
		replayStats = stats
		if replayErr != nil {
			log.Printf("[启动] ⚠ WAL回放失败: %v", replayErr)
			node.RestoreFromWAL(nil)
		} else {
			node.RestoreFromWAL(replayedLogs)
			fmt.Printf("[启动] WAL回放成功: 恢复 %d 条日志, commitIdx=%d, 完整性=%s\n",
				len(replayedLogs), replayStats.AfterCommitIdx, replayStats.Integrity)
		}
	} else {
		node.RestoreFromWAL(nil)
		replayStats.Integrity = "无WAL"
	}

	// TCX-Ⅳ: 启动WAL门禁健康探测
	if pipeline != nil && pipeline.Storage() != nil {
		dataDir := filepath.Dir(pipelineCfg.WALPath)
		audit := NewWALGateAuditLog(nodeID, dataDir)
		gate := NewWALGate(node, audit)
		node.walGate = gate
		globalWALGate = gate
		walPath := pipelineCfg.WALPath
		gate.StartWithCheck(func() error {
			return walHealthCheck(walPath)
		})
		defer gate.Stop()
		fmt.Printf("[启动] WAL门禁已启动 (阈值=%d, 探测间隔=%v)\n", 3, 1*time.Second)
	}

	// TCX-Ⅳ: 初始化批量同步管理器
	batchSyncCfg := BatchSyncConfigFromEnv()
	node.batchSyncMgr = NewBatchSyncManager(node, batchSyncCfg)
	fmt.Printf("[启动] 批量同步管理器已初始化 (enable=%v, lagThreshold=%d, maxBatch=%d)\n",
		batchSyncCfg.Enable, batchSyncCfg.LagThreshold, batchSyncCfg.MaxBatchSize)

	// 刀三: 快照兜底路径 — 设置回调 + peerHttpAddrs
	peerHttpAddrs := make(map[string]string)
	for pid, addr := range peerAddrs {
		// 从 gRPC 地址派生 HTTP 地址: 替换端口号
		// addr 格式: "node-2:9500" → "node-2:9000"
		colonIdx := strings.LastIndex(addr, ":")
		if colonIdx >= 0 {
			peerHttpAddrs[pid] = addr[:colonIdx] + ":" + httpListen
		}
	}
	node.peerHttpAddrs = peerHttpAddrs

	if pipeline != nil && pipeline.Storage() != nil {
		storage := pipeline.Storage()
		snapshotPath := storage.WAL().path + ".snapshot.gz"

		node.getSnapshotData = func() ([]byte, int64, int64, error) {
			data, err := os.ReadFile(snapshotPath)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("读取快照文件失败: %w", err)
			}
			gz, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return nil, 0, 0, fmt.Errorf("gzip 解压失败: %w", err)
			}
			decompressed, err := io.ReadAll(gz)
			gz.Close()
			if err != nil {
				return nil, 0, 0, fmt.Errorf("快照读取失败: %w", err)
			}
			var logs []RaftLog
			if err := json.Unmarshal(decompressed, &logs); err != nil {
				return nil, 0, 0, fmt.Errorf("快照反序列化失败: %w", err)
			}
			if len(logs) == 0 {
				return nil, 0, 0, fmt.Errorf("快照为空")
			}
			last := logs[len(logs)-1]
			return decompressed, last.Index, last.Term, nil
		}

		node.installSnapshot = func(data []byte, lastIdx int64, lastTerm int64) error {
			gzData, err := compressGzip(data)
			if err != nil {
				return fmt.Errorf("gzip 压缩失败: %w", err)
			}
			if err := os.WriteFile(snapshotPath, gzData, 0644); err != nil {
				return fmt.Errorf("写入快照文件失败: %w", err)
			}
			return node.ReloadFromSnapshot(data, lastIdx, lastTerm)
		}

		fmt.Printf("[启动] 快照兜底路径已就绪 (snapshotPath=%s)\n", snapshotPath)
	}

	grpcServer := NewGRPCServer(node, grpcPort)
	if err := grpcServer.Start(); err != nil {
		log.Fatalf("gRPC 服务器启动失败: %v", err)
	}
	defer grpcServer.Stop()

	httpMux := http.NewServeMux()
	authMiddleware := NewAuthMiddleware()
	httpMux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	httpMux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if grpcServer.IsReady() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
		}
	})
	httpMux.HandleFunc("/raft/status", func(w http.ResponseWriter, r *http.Request) {
		stats := node.Stats().Snapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
	httpMux.HandleFunc("/raft/stats", func(w http.ResponseWriter, r *http.Request) {
		s := node.Stats()
		s.RLock()
		fmt.Fprintf(w,
			"id=%s state=%s term=%d leader=%s commit=%d applied=%d logs=%d peers=%d voted=%s",
			s.ID, s.State, s.Term, s.LeaderID, s.CommitIndex, s.LastApplied,
			s.LogCount, s.PeerCount, s.VotedFor)
		s.RUnlock()
		// R-04修复C: 输出 follower gap + 降级状态
		gaps := node.FollowerGaps()
		degraded := node.DegradedFollowers()
		fmt.Fprintf(w, " gaps=%v degraded=%v", gaps, degraded)
	})
	// 刀三: 快照兜底路径 — follower 接收 leader 发来的快照
	httpMux.HandleFunc("/raft/install-snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if node.installSnapshot == nil {
			http.Error(w, `{"error":"installSnapshot not configured"}`, http.StatusServiceUnavailable)
			return
		}
		var req struct {
			SnapshotData      []byte `json:"snapshot_data"`
			LastIncludedIndex int64  `json:"last_included_index"`
			LastIncludedTerm  int64  `json:"last_included_term"`
			LeaderCommit      int64  `json:"leader_commit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"decode failed: %v"}`, err), http.StatusBadRequest)
			return
		}
		if err := node.installSnapshot(req.SnapshotData, req.LastIncludedIndex, req.LastIncludedTerm); err != nil {
			node.logf("[raft/%s] 快照安装失败: %v", nodeID, err)
			http.Error(w, fmt.Sprintf(`{"error":"install failed: %v"}`, err), http.StatusInternalServerError)
			return
		}
		node.logf("[raft/%s] 快照安装成功: lastIncludedIndex=%d, lastIncludedTerm=%d, leaderCommit=%d",
			nodeID, req.LastIncludedIndex, req.LastIncludedTerm, req.LeaderCommit)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	})
	idemTable := NewIdemTokenTable()
	httpMux.HandleFunc("/raft/propose", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		token := r.URL.Query().Get("idem_token")
		w.Header().Set("Content-Type", "application/json")
		if token != "" {
			entry, duplicate := idemTable.GetOrCreate(token)
			if duplicate {
				<-entry.done
				if entry.err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": entry.err.Error(), "duplicate": true})
				} else {
					json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "index": entry.index, "duplicate": true})
				}
				return
			}
			index, perr := node.Propose(body)
			idemTable.SetResult(token, index, perr)
			if perr != nil {
				idemTable.Remove(token)
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": perr.Error()})
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "index": index})
			}
			return
		}
		index, err := node.Propose(body)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "index": index})
		}
	})
	httpMux.HandleFunc("/idem/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"size": idemTable.Size(), "capacity": idemTable.capacity})
	})
	httpMux.HandleFunc("/raft/get", func(w http.ResponseWriter, r *http.Request) {
		indexStr := r.URL.Query().Get("index")
		index, err := strconv.ParseInt(indexStr, 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid index"}`, http.StatusBadRequest)
			return
		}
		entry, ok := node.GetLog(index)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"found": false})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"found":   true,
				"index":   entry.Index,
				"term":    entry.Term,
				"command": entry.Command,
			})
		}
	})
	if pipeline != nil {
		httpMux.HandleFunc("/pipeline/stats", func(w http.ResponseWriter, r *http.Request) {
			stats := pipeline.Stats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
		})
	}

	httpMux.HandleFunc("/replay/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(replayStats)
	})

	// 授权与降级状态自证端点（运维可观测，供甲方独立核验当前运行模式）
	httpMux.HandleFunc("/license/status", func(w http.ResponseWriter, r *http.Request) {
		dg, reason := IsDegradedMode()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"fail_mode":       failMode,
			"fail_mode_env":   LicenseFailModeEnvKey,
			"fail_mode_raw":   LicenseFailModeRaw(),
			"default_mode":    DefaultLicenseFailMode,
			"degraded":        dg,
			"degraded_reason": reason,
			"semantics": map[string]string{
				"closed": "授权失败即拒绝启动(Fail-Closed)",
				"open":   "授权失败降级为只读运行(Fail-Open)",
			},
		})
	})

	// 国密 SM3 防篡改链自证端点（运维可观测，供甲方独立核验国密能力真实生效）
	httpMux.HandleFunc("/sm3/status", func(w http.ResponseWriter, r *http.Request) {
		st := SM3Status()
		if globalUnifiedSM != nil {
			globalUnifiedSM.mu.RLock()
			st["chain_head"] = fmt.Sprintf("%x", globalUnifiedSM.LastSM3Hash)
			globalUnifiedSM.mu.RUnlock()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	})

	// gRPC 延迟统计端点（Histogram JSON + Prometheus 格式原生直采）
	httpMux.HandleFunc("/latency/stats", handleLatencyStats)
	httpMux.HandleFunc("/latency/metrics", handleLatencyPrometheus)

	// batch15: 统一 Prometheus /metrics 端点 + 限流 + 鉴权
	var metricsCollector *MetricsCollector
	var rateLimiter *TokenBucketLimiter

	rateLimiter = NewTokenBucketLimiter(256, 8000)
	if v := os.Getenv("RATE_LIMIT_ENABLED"); v == "true" || v == "1" {
		rateLimiter.Enable()
	}
	var snapSched *SnapshotScheduler
	if pipeline != nil {
		snapSched = pipeline.scheduler
	}
	metricsCollector = NewMetricsCollector(node, snapSched, rateLimiter)
	httpMux.HandleFunc("/metrics", authMiddleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprint(w, metricsCollector.RenderPrometheus())
	}))

	// V2.3: 动态成员变更 HTTP 端点
	httpMux.HandleFunc("/cluster/add", HandleAddNode(node))
	httpMux.HandleFunc("/cluster/remove", HandleRemoveNode(node))
	httpMux.HandleFunc("/cluster/members", HandleClusterMembers(node))

	httpSrv := &http.Server{Addr: fmt.Sprintf("%s:%s", httpBind, httpListen), Handler: httpMux}

	go func() { _ = http.ListenAndServe("127.0.0.1:9600", nil) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		fmt.Printf("[启动] gRPC 服务监听 :%s\n", grpcPort)
		<-ctx.Done()
		grpcServer.Stop()
		return nil
	})

	g.Go(func() error {
		fmt.Println("[启动] Raft 状态机开始运行")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[recover] goroutine panic: %v\n", r)
				}
			}()
			node.Run()
			cancel()
		}()
		<-ctx.Done()
		node.Shutdown()
		return nil
	})

	g.Go(func() error {
		fmt.Printf("[启动] HTTP API 服务监听 :%s\n", httpListen)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[recover] goroutine panic: %v\n", r)
				}
			}()
			<-ctx.Done()
			shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
			defer sc()
			httpSrv.Shutdown(shutdownCtx)
		}()
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("HTTP 服务异常: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-sigCh:
			fmt.Printf("\n[信号] 收到 %v，开始优雅关闭...\n", sig)
			cancel()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	g.Go(func() error {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				s := node.Stats()
				s.RLock()
				fmt.Printf("[状态] id=%s state=%s term=%d leader=%s commit=%d peers=%d\n",
					s.ID, s.State, s.Term, s.LeaderID, s.CommitIndex, s.PeerCount)
				s.RUnlock()
				if pipeline != nil {
					ps := pipeline.Stats()
					fmt.Printf("[管线] committed=%d walErr=%d sinkErr=%d sinkWritten=%d fallback=%d healthy=%v\n",
						ps.Committed, ps.WALErrors, ps.SinkErrors,
						ps.SinkWritten, ps.SinkFallbackLen, ps.SinkDBHealthy)
				}
			}
		}
	})

	fmt.Println(strings.Repeat("═", 60))
	fmt.Println("  所有服务已就绪，等待外部指令接入")
	fmt.Println(strings.Repeat("═", 60))

	// 探针3(M3): 节点内存自监控 — RSS/heap 超 4GB 时自动触发 heap dump
	go func() {
		var ms runtime.MemStats
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		dumped := false
		for {
			select {
			case <-ticker.C:
				runtime.ReadMemStats(&ms)
				if ms.Alloc > 4*1024*1024*1024 && !dumped {
					dumpPath := fmt.Sprintf("/tmp/dump-%s-%d.pb", nodeID, time.Now().Unix())
					f, err := os.Create(dumpPath)
					if err != nil {
						log.Printf("[DUMP] heap dump 创建失败: %v", err)
						continue
					}
					pprof.WriteHeapProfile(f)
					f.Close()
					log.Printf("[DUMP] heap dump 已写入: %s (Alloc=%d MiB)", dumpPath, ms.Alloc/1024/1024)
					dumped = true
				}
			}
		}
	}()

	// 降级只读模式运行期警示横幅（Fail-Open 专属，closed 模式永不进入此分支）
	if dg, dgReason := IsDegradedMode(); dg {
		fmt.Println(strings.Repeat("!", 60))
		fmt.Println("  ⚠  降级只读模式运行中（Fail-Open）")
		fmt.Printf("  原因: %s\n", dgReason)
		fmt.Println("  约束: 写入请求与集群成员变更已被拒绝")
		fmt.Println("         Raft 选举 / 心跳 / 只读查询保持正常")
		fmt.Println("  法务: 仅在《授权到期降级只读补充条款》生效下允许")
		fmt.Println(strings.Repeat("!", 60))
	}

	if err := g.Wait(); err != nil && err != context.Canceled {
		log.Printf("服务异常退出: %v", err)
		os.Exit(1)
	}
	fmt.Println("[退出] 所有服务已安全关闭")
}

func envOr(key, flagVal, defaultVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parsePeerAddrs(raw string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type stdLogger struct{ prefix string }

func (l *stdLogger) Printf(format string, v ...interface{}) {
	log.Printf("%s %s", l.prefix, fmt.Sprintf(format, v...))
}
