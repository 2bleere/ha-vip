package vip

import (
    "log"
    "net"
    "os"
    "os/exec"
    "strings"
    "sync"
    "time"

    "github.com/2bleere/ha-vip/internal/config"
    "github.com/2bleere/ha-vip/internal/election"
)

type VIPManager struct {
    cfg           *config.Config
    isAssigned    bool
    mu            sync.RWMutex
    stopCh        chan struct{}
    fastCheck     bool
    isNonRoot     bool  // Track if running as non-root user
}

func NewVIPManager(cfg *config.Config) *VIPManager {
    // Detect if running as non-root user
    isNonRoot := os.Getuid() != 0
    
    if isNonRoot {
        log.Printf("VIP Manager: Running as non-root user (UID: %d), will use fallback ARP methods", os.Getuid())
    } else {
        log.Printf("VIP Manager: Running as root user, will use optimal ARP methods")
    }
    
    return &VIPManager{
        cfg:       cfg,
        stopCh:    make(chan struct{}),
        isNonRoot: isNonRoot,
    }
}

func (v *VIPManager) Stop() {
    close(v.stopCh)
}

func (v *VIPManager) MonitorLeadership(e *election.Election) {
    log.Printf("VIP Manager: Starting leadership monitoring")
    
    // Start with immediate check
    v.checkAndUpdateVIP(e)
    
    // Listen for leadership changes for immediate response
    leaderChangeChan := e.GetLeaderChangeChan()
    
    // Use shorter polling interval for backup
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case newLeader := <-leaderChangeChan:
            // Immediate response to leadership change
            log.Printf("VIP Manager: Leadership change detected - new leader: %s", newLeader)
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
            log.Printf("VIP Manager: Stop signal received")
            return
        }
    }
}

func (v *VIPManager) checkAndUpdateVIP(e *election.Election) {
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
    
    // Send gratuitous ARP to notify network of VIP assignment
    go v.sendGratuitousARP()
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

// sendGratuitousARP sends gratuitous ARP packets to notify the network
// of the VIP assignment, ensuring fast failover
func (v *VIPManager) sendGratuitousARP() {
    // Extract IP address from VIP (remove CIDR notation if present)
    vipAddr := strings.Split(v.cfg.VIP, "/")[0]
    
    // Parse IP address
    ip := net.ParseIP(vipAddr)
    if ip == nil {
        log.Printf("Invalid VIP address for gratuitous ARP: %s", vipAddr)
        return
    }
    
    // Only send gratuitous ARP for IPv4
    if ip.To4() == nil {
        log.Printf("Gratuitous ARP not supported for IPv6 address: %s", vipAddr)
        return
    }
    
    log.Printf("Sending gratuitous ARP for VIP %s on interface %s", vipAddr, v.cfg.Interface)
    
    // Check if running as non-root and skip arping attempts
    if v.isNonRoot {
        log.Printf("Non-root execution detected")
        v.sendPingBroadcast(vipAddr)
        return
    }
    
    // Method 1: Use arping if available (most reliable) - only for root users
    if v.sendArping(vipAddr) {
        return
    }
    
    // Method 2: Fallback to ping broadcast (triggers ARP)
    v.sendPingBroadcast(vipAddr)
}

// sendArping sends gratuitous ARP using the arping tool (root users only)
func (v *VIPManager) sendArping(vipAddr string) bool {
    // This method should only be called for root users
    // Try arping with different approaches for maximum compatibility
    
    // Method 1: Try arping with gratuitous announce flag
    cmd := exec.Command("arping", "-A", "-c", "3", "-I", v.cfg.Interface, vipAddr)
    if err := cmd.Run(); err == nil {
        log.Printf("Sent gratuitous ARP using arping -A for %s", vipAddr)
        return true
    }
    
    // Method 2: Try arping with unsolicited flag
    cmd = exec.Command("arping", "-U", "-c", "3", "-I", v.cfg.Interface, vipAddr)
    if err := cmd.Run(); err == nil {
        log.Printf("Sent gratuitous ARP using arping -U for %s", vipAddr)
        return true
    }
    
    // Method 3: Try basic arping
    cmd = exec.Command("arping", "-c", "1", "-I", v.cfg.Interface, vipAddr)
    if err := cmd.Run(); err == nil {
        log.Printf("Sent ARP using arping (basic mode) for %s", vipAddr)
        return true
    }
    
    // Method 4: Check if we can use ip command instead
    if v.sendIPNeighborAnnounce(vipAddr) {
        return true
    }
    
    log.Printf("arping not available or failed for interface %s", v.cfg.Interface)
    return false
}

// sendIPNeighborAnnounce uses ip command to send neighbor announcements
func (v *VIPManager) sendIPNeighborAnnounce(vipAddr string) bool {
    // Use ip command to manipulate neighbor table (may work with CAP_NET_ADMIN)
    // First, try to add a temporary neighbor entry, then delete it to trigger announcement
    cmd := exec.Command("ip", "neigh", "add", vipAddr, "lladdr", "00:00:00:00:00:00", "dev", v.cfg.Interface)
    if err := cmd.Run(); err == nil {
        // Delete the entry to clean up and potentially trigger announcements
        cmd = exec.Command("ip", "neigh", "del", vipAddr, "dev", v.cfg.Interface)
        cmd.Run() // Ignore error on cleanup
        log.Printf("Sent neighbor announcement using ip command for %s", vipAddr)
        return true
    }
    
    return false
}

// sendPingBroadcast sends pings to trigger ARP learning (works for non-root)
func (v *VIPManager) sendPingBroadcast(vipAddr string) {
    log.Printf("Using fallback methods for ARP announcement (non-root compatible)")
    
    // Method 1: Ping the VIP itself to establish ARP entry
    cmd := exec.Command("ping", "-c", "1", "-W", "1", vipAddr)
    if err := cmd.Run(); err == nil {
        log.Printf("Sent self-ping to establish ARP for %s", vipAddr)
    }
    
    // Method 2: Get network information for broadcast ping
    v.sendNetworkBroadcast(vipAddr)
    
    // Method 3: Send pings to common gateway addresses to announce presence
    v.sendGatewayPings(vipAddr)
}

// sendNetworkBroadcast sends broadcast ping to the network
func (v *VIPManager) sendNetworkBroadcast(vipAddr string) {
    // Get network information for broadcast address
    iface, err := net.InterfaceByName(v.cfg.Interface)
    if err != nil {
        log.Printf("Failed to get interface %s: %v", v.cfg.Interface, err)
        return
    }
    
    addrs, err := iface.Addrs()
    if err != nil {
        log.Printf("Failed to get addresses for interface %s: %v", v.cfg.Interface, err)
        return
    }
    
    // Find the subnet to calculate broadcast address
    for _, addr := range addrs {
        ipNet, ok := addr.(*net.IPNet)
        if !ok {
            continue
        }
        
        if ipNet.IP.To4() == nil {
            continue // Skip IPv6
        }
        
        // Calculate broadcast address
        broadcast := make(net.IP, len(ipNet.IP.To4()))
        for i := range ipNet.IP.To4() {
            broadcast[i] = ipNet.IP.To4()[i] | ^ipNet.Mask[i]
        }
        
        // Send ping to broadcast (this will trigger ARP resolution)
        cmd := exec.Command("ping", "-c", "1", "-W", "1", "-I", v.cfg.Interface, broadcast.String())
        if err := cmd.Run(); err == nil {
            log.Printf("Sent broadcast ping from %s to %s", vipAddr, broadcast.String())
        }
        break
    }
}

// sendGatewayPings sends pings to likely gateway addresses to announce presence
func (v *VIPManager) sendGatewayPings(vipAddr string) {
    // Parse VIP to determine network
    ip := net.ParseIP(vipAddr)
    if ip == nil || ip.To4() == nil {
        return
    }
    
    // Get interface addresses to find the network
    iface, err := net.InterfaceByName(v.cfg.Interface)
    if err != nil {
        return
    }
    
    addrs, err := iface.Addrs()
    if err != nil {
        return
    }
    
    for _, addr := range addrs {
        ipNet, ok := addr.(*net.IPNet)
        if !ok || ipNet.IP.To4() == nil {
            continue
        }
        
        // Generate common gateway addresses (.1, .254) in the subnet
        network := ipNet.IP.Mask(ipNet.Mask)
        
        // Try .1 (common gateway)
        gateway1 := make(net.IP, len(network))
        copy(gateway1, network)
        gateway1[len(gateway1)-1] = 1
        
        // Try .254 (another common gateway)
        gateway254 := make(net.IP, len(network))
        copy(gateway254, network)
        gateway254[len(gateway254)-1] = 254
        
        // Ping these addresses to ensure our ARP entry is noticed
        for _, gw := range []net.IP{gateway1, gateway254} {
            if ipNet.Contains(gw) {
                cmd := exec.Command("ping", "-c", "1", "-W", "1", "-I", v.cfg.Interface, gw.String())
                if err := cmd.Run(); err == nil {
                    log.Printf("Sent gateway ping to %s to announce %s", gw.String(), vipAddr)
                }
            }
        }
        break
    }
}
