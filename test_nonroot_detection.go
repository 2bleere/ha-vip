package main

import (
    "fmt"
    "os"
    
    "github.com/2bleere/ha-vip/internal/config"
    "github.com/2bleere/ha-vip/internal/vip"
)

func main() {
    fmt.Printf("Testing non-root detection...\n")
    fmt.Printf("Current UID: %d\n", os.Getuid())
    fmt.Printf("Current GID: %d\n", os.Getgid())
    
    // Create a minimal config for testing
    cfg := &config.Config{
        VIP:       "192.168.1.100/24",
        Interface: "eth0",
    }
    
    // Create VIP manager (this should detect non-root status)
    vipMgr := vip.NewVIPManager(cfg)
    
    fmt.Printf("VIP Manager created successfully\n")
    fmt.Printf("Non-root detection test completed\n")
    
    // Clean up
    vipMgr.Stop()
}
