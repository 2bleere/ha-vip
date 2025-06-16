package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    cfg := LoadConfig("config.yaml")
    log.Println("Starting HA VIP Manager for", cfg.NodeID)

    hb := NewHeartbeat(cfg)
    go hb.Start()

    el := NewElection(cfg, hb)
    go el.Run()

    vip := NewVIPManager(cfg)
    go vip.MonitorLeadership(el)

    // Graceful shutdown
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig
    log.Println("Shutting down...")
    
    // Stop components in order
    el.Stop()
    vip.Stop()
    hb.Stop()
    
    // Release VIP if we have it
    vip.ReleaseVIP()
    
    log.Println("Shutdown complete")
}
