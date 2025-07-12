package election

import (
    "log"
    "sort"
    "sync"
    "time"

    "github.com/2bleere/ha-vip/internal/config"
    "github.com/2bleere/ha-vip/internal/heartbeat"
    "github.com/2bleere/ha-vip/internal/k8s"
)

type NodeInfo struct {
    NodeID   string
    Priority int
    Healthy  bool
}

type Election struct {
    cfg           *config.Config
    hb            *heartbeat.Heartbeat
    k8sChecker    *k8s.K8sHealthChecker
    leader        string
    mu            sync.RWMutex
    leaderChange  chan string
    stopCh        chan struct{}
    lastStatusLog time.Time
}

func NewElection(cfg *config.Config, hb *heartbeat.Heartbeat, k8sChecker *k8s.K8sHealthChecker) *Election {
    return &Election{
        cfg:          cfg,
        hb:           hb,
        k8sChecker:   k8sChecker,
        leaderChange: make(chan string, 1),
        stopCh:       make(chan struct{}),
    }
}

func (e *Election) Run() {
    // Initial election
    e.evaluate()
    
    ticker := time.NewTicker(time.Duration(e.cfg.ElectionTimeout) * time.Second)
    defer ticker.Stop()
    
    // Listen for K8s health changes if enabled
    var k8sHealthCh <-chan bool
    if e.k8sChecker != nil {
        k8sHealthCh = e.k8sChecker.GetHealthChangeChan()
        log.Printf("Election: Listening for K8s health changes")
    } else {
        log.Printf("Election: K8s health checker not available")
    }
    
    for {
        select {
        case <-ticker.C:
            log.Printf("Election: Timer triggered, re-evaluating")
            e.evaluate()
        case healthStatus := <-k8sHealthCh:
            // Immediate re-evaluation on health change
            log.Printf("Election: K8s health change detected (new status: %v), re-evaluating leadership immediately", healthStatus)
            e.evaluate()
        case <-e.stopCh:
            log.Printf("Election: Stop signal received")
            return
        }
    }
}

func (e *Election) Stop() {
    close(e.stopCh)
}

func (e *Election) GetLeaderChangeChan() <-chan string {
    return e.leaderChange
}

func (e *Election) evaluate() {
    peers := e.hb.GetPeers()
    
    // Build list of all nodes with their info
    var nodes []NodeInfo
    
    // Add local node
    localHealthy := true
    if e.cfg.K8s.Enabled && e.k8sChecker != nil {
        localHealthy = e.k8sChecker.IsHealthy()
    }
    
    nodes = append(nodes, NodeInfo{
        NodeID:   e.cfg.NodeID,
        Priority: e.cfg.Priority,
        Healthy:  localHealthy,
    })
    
    // Add peer nodes with their reported health status
    for peer, peerInfo := range peers {
        peerHealthy := peerInfo.Healthy
        
        // If peer is not in K8s mode but we are, consider them unhealthy
        if e.cfg.K8s.Enabled && !peerInfo.K8sMode {
            peerHealthy = false
        }
        
        nodes = append(nodes, NodeInfo{
            NodeID:   peer,
            Priority: peerInfo.Priority,
            Healthy:  peerHealthy,
        })
    }
    
    // Select leader based on K8s health and priority
    newLeader := e.selectLeader(nodes)
    
    e.mu.Lock()
    oldLeader := e.leader
    e.leader = newLeader
    e.mu.Unlock()
    
    // Only log when leadership actually changes or there's a significant event
    if oldLeader != newLeader {
        log.Printf("Leadership changed from %s to %s - notifying VIP manager", oldLeader, newLeader)
        
        // Log detailed election info only on leadership change
        log.Printf("Election: Leadership evaluation triggered by change")
        log.Printf("Election: Local K8s health status: %v", localHealthy)
        for peer, peerInfo := range peers {
            peerHealthy := peerInfo.Healthy
            if e.cfg.K8s.Enabled && !peerInfo.K8sMode {
                peerHealthy = false
            }
            log.Printf("Election: Peer %s - Priority: %d, Healthy: %v, K8sMode: %v (LastSeen: %v ago)", 
                peer, peerInfo.Priority, peerHealthy, peerInfo.K8sMode, time.Since(peerInfo.LastSeen).Round(time.Second))
        }
        
        select {
        case e.leaderChange <- newLeader:
        default:
            // Channel full, skip
        }
    }
    
    // Log current status periodically (every 30 seconds) or on change
    now := time.Now()
    var shouldLog bool
    e.mu.Lock()
    if e.lastStatusLog.IsZero() || now.Sub(e.lastStatusLog) >= 30*time.Second || oldLeader != newLeader {
        e.lastStatusLog = now
        shouldLog = true
    } else {
        shouldLog = false
    }
    e.mu.Unlock()
    
    if shouldLog {
        if e.cfg.K8s.Enabled {
            healthyCount := 0
            for _, node := range nodes {
                if node.Healthy {
                    healthyCount++
                }
            }
            log.Printf("Current leader: %s (K8s enabled, local healthy: %v, %d/%d nodes healthy)", 
                e.leader, localHealthy, healthyCount, len(nodes))
        } else {
            log.Printf("Current leader: %s (K8s disabled, %d total nodes)", e.leader, len(nodes))
        }
    }
}

func (e *Election) selectLeader(nodes []NodeInfo) string {
    if len(nodes) == 0 {
        return e.cfg.NodeID
    }
    
    // Enhanced debug logging
    log.Printf("Election: selectLeader called with %d nodes:", len(nodes))
    for i, node := range nodes {
        log.Printf("  Node %d: ID=%s, Priority=%d, Healthy=%v", i, node.NodeID, node.Priority, node.Healthy)
    }
    
    if !e.cfg.K8s.Enabled {
        // If K8s is disabled, use simple alphabetical sorting
        var candidates []string
        for _, node := range nodes {
            candidates = append(candidates, node.NodeID)
        }
        sort.Strings(candidates)
        return candidates[0]
    }
    
    // K8s-aware leader selection
    
    // Step 1: Find healthy nodes
    var healthyNodes []NodeInfo
    for _, node := range nodes {
        if node.Healthy {
            healthyNodes = append(healthyNodes, node)
        }
    }
    
    log.Printf("Election: Found %d healthy nodes out of %d total", len(healthyNodes), len(nodes))
    for i, node := range healthyNodes {
        log.Printf("  Healthy Node %d: ID=%s, Priority=%d", i, node.NodeID, node.Priority)
    }
    
    // Step 2: If we have healthy nodes, select the highest priority (lowest number) among them
    if len(healthyNodes) > 0 {
        // Sort by priority (ascending), then by NodeID for consistency
        sort.Slice(healthyNodes, func(i, j int) bool {
            if healthyNodes[i].Priority == healthyNodes[j].Priority {
                return healthyNodes[i].NodeID < healthyNodes[j].NodeID
            }
            return healthyNodes[i].Priority < healthyNodes[j].Priority
        })
        
        leader := healthyNodes[0].NodeID
        log.Printf("Selected healthy leader: %s (priority: %d)", leader, healthyNodes[0].Priority)
        return leader
    }
    
    // Step 3: If no healthy nodes, select highest priority (lowest number) regardless of health
    log.Printf("Election: No healthy nodes found, selecting based on priority alone")
    sort.Slice(nodes, func(i, j int) bool {
        if nodes[i].Priority == nodes[j].Priority {
            return nodes[i].NodeID < nodes[j].NodeID
        }
        return nodes[i].Priority < nodes[j].Priority
    })
    
    leader := nodes[0].NodeID
    log.Printf("No healthy nodes, selected highest priority leader: %s (priority: %d)", leader, nodes[0].Priority)
    return leader
}

func (e *Election) IsLeader() bool {
    e.mu.RLock()
    defer e.mu.RUnlock()
    return e.leader == e.cfg.NodeID
}
