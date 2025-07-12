package heartbeat

import (
    "encoding/json"
    "log"
    "net"
    "sync"
    "time"

    "github.com/2bleere/ha-vip/internal/config"
    "github.com/2bleere/ha-vip/internal/k8s"
)

type HeartbeatMessage struct {
    NodeID   string `json:"node_id"`
    Priority int    `json:"priority"`
    Healthy  bool   `json:"healthy"`
    K8sMode  bool   `json:"k8s_mode"`
}

type PeerInfo struct {
    LastSeen time.Time
    Priority int
    Healthy  bool
    K8sMode  bool
}

type Heartbeat struct {
    cfg            *config.Config
    k8sChecker     *k8s.K8sHealthChecker
    peers          map[string]PeerInfo
    mu             sync.Mutex
    stopCh         chan struct{}
    conn           *net.UDPConn
    lastSentHealth map[string]bool
}

func NewHeartbeat(cfg *config.Config, k8sChecker *k8s.K8sHealthChecker) *Heartbeat {
    return &Heartbeat{
        cfg:            cfg,
        k8sChecker:     k8sChecker,
        peers:          make(map[string]PeerInfo),
        stopCh:         make(chan struct{}),
        lastSentHealth: make(map[string]bool),
    }
}

func (h *Heartbeat) Start() {
    go h.listen()
    ticker := time.NewTicker(time.Duration(h.cfg.HeartbeatInterval) * time.Second)
    for {
        select {
        case <-ticker.C:
            h.send()
        case <-h.stopCh:
            return
        }
    }
}

func (h *Heartbeat) send() {
    // Create heartbeat message with current health status
    healthy := true
    if h.cfg.K8s.Enabled && h.k8sChecker != nil {
        healthy = h.k8sChecker.IsHealthy()
    }
    
    msg := HeartbeatMessage{
        NodeID:   h.cfg.NodeID,
        Priority: h.cfg.Priority,
        Healthy:  healthy,
        K8sMode:  h.cfg.K8s.Enabled,
    }
    
    // Only log heartbeat when health status changes
    h.mu.Lock()
    lastHealthy, exists := h.lastSentHealth[h.cfg.NodeID]
    if !exists || lastHealthy != healthy {
        h.lastSentHealth[h.cfg.NodeID] = healthy
        log.Printf("Heartbeat: Health status changed for %s - Priority: %d, Healthy: %v, K8sMode: %v", 
            h.cfg.NodeID, h.cfg.Priority, healthy, h.cfg.K8s.Enabled)
    }
    h.mu.Unlock()
    
    msgBytes, err := json.Marshal(msg)
    if err != nil {
        log.Printf("Failed to marshal heartbeat message: %v", err)
        return
    }
    
    for _, peer := range h.cfg.Peers {
        conn, err := net.Dial("udp", peer)
        if err == nil {
            conn.Write(msgBytes)
            conn.Close()
        }
    }
}

func (h *Heartbeat) listen() {
    addr := net.UDPAddr{Port: h.cfg.Port, IP: net.IPv4zero}
    conn, err := net.ListenUDP("udp", &addr)
    if err != nil {
        log.Printf("Failed to start UDP listener: %v", err)
        return
    }
    h.conn = conn
    
    buf := make([]byte, 1024)
    for {
        select {
        case <-h.stopCh:
            return
        default:
            // Reduced read timeout for faster response
            conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
            n, _, err := conn.ReadFromUDP(buf)
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    continue
                }
                log.Printf("UDP read error: %v", err)
                continue
            }
            
            // Try to parse as JSON heartbeat message
            var msg HeartbeatMessage
            if err := json.Unmarshal(buf[:n], &msg); err != nil {
                // Fallback to old format (just node ID)
                peerID := string(buf[:n])
                h.mu.Lock()
                h.peers[peerID] = PeerInfo{
                    LastSeen: time.Now(),
                    Priority: 100, // Default low priority
                    Healthy:  !h.cfg.K8s.Enabled, // Healthy if K8s is disabled
                    K8sMode:  false,
                }
                h.mu.Unlock()
                continue
            }
            
            h.mu.Lock()
            // Enhanced logging for received heartbeats
            oldPeer, existed := h.peers[msg.NodeID]
            h.peers[msg.NodeID] = PeerInfo{
                LastSeen: time.Now(),
                Priority: msg.Priority,
                Healthy:  msg.Healthy,
                K8sMode:  msg.K8sMode,
            }
            
            if !existed {
                log.Printf("Heartbeat: New peer discovered - %s (Priority: %d, Healthy: %v, K8sMode: %v)", 
                    msg.NodeID, msg.Priority, msg.Healthy, msg.K8sMode)
            } else if oldPeer.Healthy != msg.Healthy {
                log.Printf("Heartbeat: Peer %s health changed from %v to %v (Priority: %d, K8sMode: %v)", 
                    msg.NodeID, oldPeer.Healthy, msg.Healthy, msg.Priority, msg.K8sMode)
            }
            h.mu.Unlock()
        }
    }
}

func (h *Heartbeat) Stop() {
    close(h.stopCh)
    if h.conn != nil {
        h.conn.Close()
    }
}

func (h *Heartbeat) GetPeers() map[string]PeerInfo {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    copy := make(map[string]PeerInfo)
    // Reduced timeout for faster failure detection (2x instead of 3x)
    timeout := time.Duration(h.cfg.HeartbeatInterval*2) * time.Second
    now := time.Now()
    
    // Only include active peers (within timeout period)
    for k, v := range h.peers {
        if now.Sub(v.LastSeen) <= timeout {
            copy[k] = v
        }
    }
    
    // Clean up stale peers from the original map
    for k, v := range h.peers {
        if now.Sub(v.LastSeen) > timeout {
            delete(h.peers, k)
        }
    }
    
    return copy
}
