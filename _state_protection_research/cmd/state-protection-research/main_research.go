package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sp "state_protection_research"
)

var (
	Version   = "v2.4-research"
	BuildTime = "unknown"
)

func main() {
	id := flag.String("id", "", "节点唯一 ID (环境变量: NODE_ID)")

	httpPort := flag.String("http", "", "HTTP API 端口 (环境变量: HTTP_PORT)")
	peersRaw := flag.String("peers", "", "Peer 列表 (环境变量: PEERS)")
	leaderID := flag.String("leader", "", "初始 Leader ID (环境变量: LEADER_ID)")
	flag.Parse()

	nodeID := envOr("NODE_ID", *id, "node-1")
	httpListen := envOr("HTTP_PORT", *httpPort, "9000")
	peerList := envOr("PEERS", *peersRaw, "")
	initLeader := envOr("LEADER_ID", *leaderID, "")

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  岱境235 State Protection Research Build\n")
	fmt.Printf("  版本: %s  构建: %s\n", Version, BuildTime)
	fmt.Printf("  [研究镜像] 此镜像与V2.2-S商业镜像完全隔离\n")
	fmt.Println(strings.Repeat("=", 60))

	stateBinPath := envOr("STATE_BIN_PATH", "", "/app/data/state.bin")
	stateHmacPath := envOr("STATE_HMAC_PATH", "", "/app/data/state.bin.hmac")
	hmacKeyPath := envOr("STATE_HMAC_KEY_PATH", "", "/app/data/state_hmac_key.bin")
	rsaPubKeyPath := envOr("RSA_PUBLIC_KEY_PATH", "", "/app/keys/public.pem")
	snapshotIntervalMs := envOrInt("STATE_SNAPSHOT_INTERVAL_MS", 100)
	resyncTimeoutMs := envOrInt("STATE_RESYNC_TIMEOUT_MS", 10000)
	resyncMaxRetry := envOrInt("STATE_RESYNC_MAX_RETRY", 3)
	resyncRetryIntervalMs := envOrInt("STATE_RESYNC_RETRY_INTERVAL_MS", 1000)

	peers := parsePeers(peerList)

	fmt.Printf("[启动] 节点 ID: %s, HTTP: %s\n", nodeID, httpListen)
	fmt.Printf("[启动] Peer 列表 (%d 个): %v\n", len(peers), peers)
	fmt.Printf("[启动] state.bin: %s\n", stateBinPath)
	fmt.Printf("[启动] state.bin.hmac: %s\n", stateHmacPath)

	node := sp.NewSimpleRaftNode(nodeID, peers)
	if initLeader != "" {
		if nodeID == initLeader {
			node.BecomeLeader()
			fmt.Printf("[启动] 节点 %s 成为初始 Leader\n", nodeID)
		} else {
			node.SetLeaderID(initLeader)
			node.SetLogCaughtUp(true)
			fmt.Printf("[启动] 节点 %s 初始 Follower, Leader=%s\n", nodeID, initLeader)
		}
	} else {
		node.SetLogCaughtUp(true)
	}

	logger := &sp.StdLogger{Prefix: ""}

	cfg := sp.StateProtectorConfig{
		StateBinPath:       stateBinPath,
		StateHmacPath:      stateHmacPath,
		HmacKeyPath:        hmacKeyPath,
		RsaPubKeyPath:      rsaPubKeyPath,
		SnapshotIntervalMs: snapshotIntervalMs,
		ResyncConfig: sp.ResyncConfig{
			TimeoutMs:       resyncTimeoutMs,
			MaxRetry:        resyncMaxRetry,
			RetryIntervalMs: resyncRetryIntervalMs,
		},
	}

	protector, err := sp.NewStateProtector(cfg, logger)
	if err != nil {
		log.Fatalf("StateProtector 初始化失败: %v", err)
	}

	fmt.Println("[启动] 开始加载与校验 state.bin...")
	result, err := protector.LoadAndVerify()
	if err != nil {
		log.Fatalf("LoadAndVerify 失败: %v", err)
	}

	fmt.Printf("[启动] 完整性校验结果: %s\n", result.Status)

	switch result.Status {
	case sp.INTEGRITY_OK:
		if result.VoteRecord != nil {
			node.SetTerm(result.VoteRecord.Term)
			node.SetVotedFor(result.VoteRecord.VotedFor)
			fmt.Printf("[启动] 加载持久化状态: term=%d, votedFor=%s\n", result.VoteRecord.Term, result.VoteRecord.VotedFor)
		}
	case sp.INTEGRITY_LEGACY_NO_HMAC:
		if result.VoteRecord != nil {
			node.SetTerm(result.VoteRecord.Term)
			node.SetVotedFor(result.VoteRecord.VotedFor)
			fmt.Printf("[启动] 存量兼容: term=%d, votedFor=%s (HMAC已初始化)\n", result.VoteRecord.Term, result.VoteRecord.VotedFor)
		}
	case sp.INTEGRITY_NEW_NODE:
		fmt.Println("[启动] 新节点，无持久化状态")
	case sp.INTEGRITY_TAMPERED:
		fmt.Println("[启动] 检测到篡改！进入 local-cache-corrupted 状态")
		node.SetLogCaughtUp(false)
		fmt.Println("[启动] 触发重同步恢复...")
		if protector.HandleTampered(node) {
			fmt.Println("[启动] 重同步成功，节点恢复")
			node.SetLogCaughtUp(true)
		} else {
			fmt.Println("[启动] 重同步失败，节点保持 corrupted 状态")
		}
	}

	stopSnapshot := protector.StartSnapshotLoop(node)
	defer stopSnapshot()

	mux := http.NewServeMux()
	node.RegisterHTTPHandlers(mux)
	protector.Metrics().RegisterHTTPHandler(mux)

	mux.HandleFunc("/raft/status", func(w http.ResponseWriter, r *http.Request) {
		status := node.StatusJSON()
		status["integrity_status"] = result.Status.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", httpListen),
		Handler: mux,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Printf("\n[信号] 收到终止信号，开始优雅关闭...\n")
		cancel()
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Printf("[状态] id=%s term=%d leader=%s voted=%s caught_up=%t\n",
					node.ID(), node.Term(), node.LeaderID(), node.GetVotedFor(), node.IsLogCaughtUp())
			}
		}
	}()

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  State Protection Research 节点已就绪")
	fmt.Println(strings.Repeat("=", 60))

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP 服务异常: %v", err)
	}

	protector.Shutdown()
	fmt.Println("[退出] 节点已安全关闭")
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

func envOrInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return defaultVal
}

func parsePeers(raw string) []string {
	var peers []string
	for _, pair := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) >= 1 && kv[0] != "" {
			peers = append(peers, strings.TrimSpace(kv[0]))
		}
	}
	return peers
}
