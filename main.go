package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
)

// These values are set at build time using -ldflags
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
    // Parse command-line flags
    configFile := flag.String("config", "config.yaml", "Path to configuration file")
    showVersion := flag.Bool("version", false, "Show version information and exit")
    flag.Parse()
    
    // Show version if requested
    if *showVersion {
        fmt.Printf("HA VIP Manager v%s (commit: %s, built: %s)\n", version, commit, date)
        os.Exit(0)
    }
    
    cfg := LoadConfig(*configFile)
    log.Printf("Starting HA VIP Manager v%s for %s", version, cfg.NodeID)

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
