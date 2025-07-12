package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/2bleere/ha-vip/internal/config"
    "github.com/2bleere/ha-vip/internal/election"
    "github.com/2bleere/ha-vip/internal/heartbeat"
    "github.com/2bleere/ha-vip/internal/k8s"
    "github.com/2bleere/ha-vip/internal/vip"
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
    
    cfg := config.LoadConfig(*configFile)
    log.Printf("Starting HA VIP Manager v%s for %s", version, cfg.NodeID)

    // Initialize K8s health checker if enabled
    var k8sChecker *k8s.K8sHealthChecker
    if cfg.K8s.Enabled {
        log.Printf("Kubernetes integration enabled for API server: %s", cfg.K8s.APIServer)
        k8sChecker = k8s.NewK8sHealthChecker(cfg)
        if k8sChecker != nil {
            go k8sChecker.Start()
        }
    } else {
        log.Println("Kubernetes integration disabled")
    }

    hb := heartbeat.NewHeartbeat(cfg, k8sChecker)
    go hb.Start()

    el := election.NewElection(cfg, hb, k8sChecker)
    go el.Run()

    vipManager := vip.NewVIPManager(cfg)
    go vipManager.MonitorLeadership(el)

    // Graceful shutdown
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig
    log.Println("Shutting down...")
    
    // Stop components in order
    el.Stop()
    vipManager.Stop()
    hb.Stop()
    
    // Stop K8s health checker if it was started
    if k8sChecker != nil {
        k8sChecker.Stop()
    }
    
    // Release VIP if we have it
    vipManager.ReleaseVIP()
    
    log.Println("Shutdown complete")
}
