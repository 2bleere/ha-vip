package main

import (
    "log"
    "os/exec"
    "sync"
    "time"
)

type VIPManager struct {
    cfg           *Config
    isAssigned    bool
    mu            sync.RWMutex
    stopCh        chan struct{}
    fastCheck     bool
}

func NewVIPManager(cfg *Config) *VIPManager {
    return &VIPManager{
        cfg:    cfg,
        stopCh: make(chan struct{}),
    }
}

func (v *VIPManager) Stop() {
    close(v.stopCh)
}

func (v *VIPManager) MonitorLeadership(e *Election) {
    // Start with immediate check
    v.checkAndUpdateVIP(e)
    
    // Listen for leadership changes for immediate response
    leaderChangeChan := e.GetLeaderChangeChan()
    
    // Use shorter polling interval for backup
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-leaderChangeChan:
            // Immediate response to leadership change
            log.Println("Leadership change detected, updating VIP immediately")
            v.checkAndUpdateVIP(e)
            v.fastCheck = true
            
        case <-ticker.C:
            // Regular polling as backup
            interval := 1 * time.Second
            if v.fastCheck {
                // Use faster polling for a short period after leadership change
                interval = 200 * time.Millisecond
                v.fastCheck = false
            }
            ticker.Reset(interval)
            v.checkAndUpdateVIP(e)
            
        case <-v.stopCh:
            return
        }
    }
}

func (v *VIPManager) checkAndUpdateVIP(e *Election) {
    if e.IsLeader() {
        v.AssignVIP()
    } else {
        v.ReleaseVIP()
    }
}

func (v *VIPManager) AssignVIP() {
    v.mu.Lock()
    defer v.mu.Unlock()
    
    if v.isAssigned {
        return // Already assigned
    }
    
    cmd := exec.Command("ip", "addr", "add", v.cfg.VIP, "dev", v.cfg.Interface)
    if err := cmd.Run(); err != nil {
        log.Printf("Failed to assign VIP %s: %v", v.cfg.VIP, err)
        return
    }
    v.isAssigned = true
    log.Printf("Successfully assigned VIP: %s", v.cfg.VIP)
}

func (v *VIPManager) ReleaseVIP() {
    v.mu.Lock()
    defer v.mu.Unlock()
    
    if !v.isAssigned {
        return // Already released
    }
    
    cmd := exec.Command("ip", "addr", "del", v.cfg.VIP, "dev", v.cfg.Interface)
    if err := cmd.Run(); err != nil {
        log.Printf("Failed to release VIP %s: %v", v.cfg.VIP, err)
        return
    }
    v.isAssigned = false
    log.Printf("Successfully released VIP: %s", v.cfg.VIP)
}
