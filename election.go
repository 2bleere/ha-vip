package main

import (
    "log"
    "sort"
    "sync"
    "time"
)

type Election struct {
    cfg          *Config
    hb           *Heartbeat
    leader       string
    mu           sync.RWMutex
    leaderChange chan string
    stopCh       chan struct{}
}

func NewElection(cfg *Config, hb *Heartbeat) *Election {
    return &Election{
        cfg:          cfg,
        hb:           hb,
        leaderChange: make(chan string, 1),
        stopCh:       make(chan struct{}),
    }
}

func (e *Election) Run() {
    // Initial election
    e.evaluate()
    
    ticker := time.NewTicker(time.Duration(e.cfg.ElectionTimeout) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            e.evaluate()
        case <-e.stopCh:
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
    candidates := []string{e.cfg.NodeID}
    for peer := range peers {
        candidates = append(candidates, peer)
    }
    sort.Strings(candidates)
    
    newLeader := candidates[0]
    
    e.mu.Lock()
    oldLeader := e.leader
    e.leader = newLeader
    e.mu.Unlock()
    
    // Notify of leadership change immediately
    if oldLeader != newLeader {
        log.Printf("Leadership changed from %s to %s", oldLeader, newLeader)
        select {
        case e.leaderChange <- newLeader:
        default:
            // Channel full, skip
        }
    }
    
    log.Println("Current leader:", e.leader)
}

func (e *Election) IsLeader() bool {
    e.mu.RLock()
    defer e.mu.RUnlock()
    return e.leader == e.cfg.NodeID
}
