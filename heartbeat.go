package main

import (
    "fmt"
    "log"
    "net"
    "sync"
    "time"
)

type Heartbeat struct {
    cfg     *Config
    peers   map[string]time.Time
    mu      sync.Mutex
    stopCh  chan struct{}
    conn    *net.UDPConn
}

func NewHeartbeat(cfg *Config) *Heartbeat {
    return &Heartbeat{
        cfg:    cfg,
        peers:  make(map[string]time.Time),
        stopCh: make(chan struct{}),
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
    for _, peer := range h.cfg.Peers {
        conn, err := net.Dial("udp", peer)
        if err == nil {
            fmt.Fprintf(conn, "%s", h.cfg.NodeID)
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
            peerID := string(buf[:n])
            h.mu.Lock()
            h.peers[peerID] = time.Now()
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

func (h *Heartbeat) GetPeers() map[string]time.Time {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    copy := make(map[string]time.Time)
    // Reduced timeout for faster failure detection (2x instead of 3x)
    timeout := time.Duration(h.cfg.HeartbeatInterval*2) * time.Second
    now := time.Now()
    
    // Only include active peers (within timeout period)
    for k, v := range h.peers {
        if now.Sub(v) <= timeout {
            copy[k] = v
        }
    }
    
    // Clean up stale peers from the original map
    for k, v := range h.peers {
        if now.Sub(v) > timeout {
            delete(h.peers, k)
        }
    }
    
    return copy
}
