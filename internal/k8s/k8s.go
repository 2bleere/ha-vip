package k8s

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "io"
    "log"
    "net"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/2bleere/ha-vip/internal/config"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type K8sHealthChecker struct {
    cfg               *config.Config
    restConfig        *rest.Config
    client            kubernetes.Interface
    httpClient        *http.Client
    mu                sync.RWMutex
    healthy           bool
    stopCh            chan struct{}
    healthCh          chan bool
    stableHealthy     bool
    healthHistory     []bool
    healthChangedAt   time.Time
    lastStateChange   time.Time
}

func NewK8sHealthChecker(cfg *config.Config) *K8sHealthChecker {
    if !cfg.K8s.Enabled {
        return nil
    }

    log.Printf("Initializing K8s health checker with in-cluster: %v", cfg.K8s.InCluster)
    
    var restConfig *rest.Config
    var err error

    if cfg.K8s.InCluster {
        // Use in-cluster configuration
        log.Printf("Using in-cluster Kubernetes configuration")
        restConfig, err = rest.InClusterConfig()
        if err != nil {
            log.Printf("Failed to create in-cluster config: %v", err)
            return nil
        }
    } else {
        // Use external configuration
        log.Printf("Using external Kubernetes configuration with API server: %s", cfg.K8s.APIServer)
        
        // Validate configuration
        if cfg.K8s.APIServer == "" || cfg.K8s.APIServer == "https://YOUR-API-SERVER:6443" {
            log.Printf("ERROR: K8s API server not properly configured. Please set a real API server URL.")
            log.Printf("Current value: %s", cfg.K8s.APIServer)
            return nil
        }

        // Create REST config for client-go
        restConfig = &rest.Config{
            Host:    cfg.K8s.APIServer,
            Timeout: 5 * time.Second, // Add explicit timeout
        }

        // Setup authentication
        if cfg.K8s.Token != "" {
            restConfig.BearerToken = cfg.K8s.Token
        }

        // Setup TLS configuration
        if cfg.K8s.CACert != "" {
            caCert, err := os.ReadFile(cfg.K8s.CACert)
            if err != nil {
                log.Printf("Warning: Failed to read K8s CA cert %s: %v", cfg.K8s.CACert, err)
            } else {
                restConfig.CAData = caCert
            }
        } else {
            // Skip TLS verification if no CA cert provided (not recommended for production)
            restConfig.Insecure = true
        }
    }

    // Create Kubernetes client
    clientset, err := kubernetes.NewForConfig(restConfig)
    if err != nil {
        log.Printf("Failed to create Kubernetes client: %v", err)
        return nil
    }

    // Create HTTP client for /readyz endpoint
    httpClient := &http.Client{
        Timeout: 5 * time.Second,
    }

    // Setup TLS for HTTP client based on the REST config
    if cfg.K8s.InCluster {
        // For in-cluster, use the service account's CA
        if restConfig.CAData != nil {
            caCertPool := x509.NewCertPool()
            caCertPool.AppendCertsFromPEM(restConfig.CAData)
            
            httpClient.Transport = &http.Transport{
                TLSClientConfig: &tls.Config{
                    RootCAs: caCertPool,
                },
            }
        }
    } else {
        // For external config, use the configured CA cert
        if cfg.K8s.CACert != "" {
            caCert, err := os.ReadFile(cfg.K8s.CACert)
            if err == nil {
                caCertPool := x509.NewCertPool()
                caCertPool.AppendCertsFromPEM(caCert)
                
                httpClient.Transport = &http.Transport{
                    TLSClientConfig: &tls.Config{
                        RootCAs: caCertPool,
                    },
                }
            }
        } else {
            httpClient.Transport = &http.Transport{
                TLSClientConfig: &tls.Config{
                    InsecureSkipVerify: true,
                },
            }
        }
    }

    return &K8sHealthChecker{
        cfg:             cfg,
        restConfig:      restConfig,
        client:          clientset,
        httpClient:      httpClient,
        stopCh:          make(chan struct{}),
        healthCh:        make(chan bool, 10), // Increase buffer size to prevent blocking
        stableHealthy:   true,                // Start with healthy assumption
        healthHistory:   make([]bool, 0, 3), // Keep last 3 checks for 5-second window
    }
}

func (k *K8sHealthChecker) Start() {
    if k == nil {
        return
    }

    log.Printf("Starting Kubernetes health checker for node %s", k.cfg.NodeID)

    // Initial health check
    k.checkHealth()

    // Start periodic health checking
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            k.checkHealth()
        case <-k.stopCh:
            return
        }
    }
}

func (k *K8sHealthChecker) Stop() {
    if k == nil {
        return
    }
    close(k.stopCh)
}

func (k *K8sHealthChecker) checkHealth() {
    rawHealthy := k.performHealthCheck()
    
    k.mu.Lock()
    defer k.mu.Unlock()
    
    now := time.Now()
    
    // Add to health history (keep last 3 checks for 5-second window)
    k.healthHistory = append(k.healthHistory, rawHealthy)
    if len(k.healthHistory) > 3 {
        k.healthHistory = k.healthHistory[1:]
    }
    
    // Determine stable health status with 5-second total buffer
    var stableHealthy bool
    if len(k.healthHistory) >= 2 {
        // Require 2 consecutive same results (4 seconds) to change state
        lastTwo := k.healthHistory[len(k.healthHistory)-2:]
        if lastTwo[0] == lastTwo[1] {
            stableHealthy = lastTwo[1]
        } else {
            // Not stable, keep previous stable state
            stableHealthy = k.stableHealthy
        }
    } else {
        // Not enough history, use current stable state
        stableHealthy = k.stableHealthy
    }
    
    oldStableHealthy := k.stableHealthy
    k.stableHealthy = stableHealthy
    k.healthy = stableHealthy
    
    // Only log and notify when stable health status changes
    if oldStableHealthy != stableHealthy {
        // Add minimum delay between state changes (3 seconds for total 5-second buffer)
        if k.lastStateChange.IsZero() || now.Sub(k.lastStateChange) >= 3*time.Second {
            k.lastStateChange = now
            log.Printf("K8s health stabilized for node %s: %v (was: %v) - raw checks: %v", 
                k.cfg.NodeID, stableHealthy, oldStableHealthy, k.healthHistory)
            
            select {
            case k.healthCh <- stableHealthy:
                log.Printf("Successfully sent stable health change notification")
            default:
                log.Printf("Health change notification channel full, notification skipped")
            }
        } else {
            // Reset to previous state - too soon for another change
            k.stableHealthy = oldStableHealthy
            k.healthy = oldStableHealthy
            log.Printf("K8s health change suppressed for node %s (too soon - %v since last change)", 
                k.cfg.NodeID, now.Sub(k.lastStateChange))
        }
    }
}

func (k *K8sHealthChecker) performHealthCheck() bool {
    if k == nil {
        return false
    }

    // Method 1: Basic connectivity test
    if !k.checkBasicConnectivity() {
        return false
    }

    // Method 2: /readyz endpoint is the authoritative health check for K8s API
    readyzHealthy := k.checkReadyzEndpoint()
    
    // For HA-VIP purposes, we ONLY trust the /readyz endpoint
    // If /readyz says unhealthy, the API server should not receive traffic
    if !readyzHealthy {
        return false
    }

    return true
}

func (k *K8sHealthChecker) checkBasicConnectivity() bool {
    // Get the appropriate host for connectivity check
    var apiURL string
    if k.cfg.K8s.InCluster {
        // For in-cluster, use the REST config's host
        apiURL = strings.TrimPrefix(k.restConfig.Host, "https://")
        apiURL = strings.TrimPrefix(apiURL, "http://")
    } else {
        // For external config, use the configured API server
        apiURL = strings.TrimPrefix(k.cfg.K8s.APIServer, "https://")
        apiURL = strings.TrimPrefix(apiURL, "http://")
    }
    
    // Simple TCP connectivity test
    conn, err := net.DialTimeout("tcp", apiURL, 2*time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    
    return true
}

func (k *K8sHealthChecker) checkClientGo() bool {
    // Add timeout context for client-go call
    done := make(chan bool, 1)
    var err error
    
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("K8s client-go panic recovered: %v", r)
                done <- false
            }
        }()
        
        // Try to get server version as a health check
        _, err = k.client.Discovery().ServerVersion()
        done <- err == nil
    }()
    
    // Wait for result with timeout
    select {
    case healthy := <-done:
        if !healthy && err != nil {
            log.Printf("K8s client-go health check failed for %s: %v", k.cfg.NodeID, err)
        }
        return healthy
    case <-time.After(3 * time.Second):
        log.Printf("K8s client-go health check timed out for node %s", k.cfg.NodeID)
        return false
    }
}

func (k *K8sHealthChecker) checkReadyzEndpoint() bool {
    // Construct readyz URL
    var readyzURL string
    if k.cfg.K8s.InCluster {
        // For in-cluster, use the REST config's host
        readyzURL = strings.TrimSuffix(k.restConfig.Host, "/") + "/readyz"
    } else {
        // For external config, use the configured API server
        readyzURL = strings.TrimSuffix(k.cfg.K8s.APIServer, "/") + "/readyz"
    }
    
    req, err := http.NewRequest("GET", readyzURL, nil)
    if err != nil {
        return false
    }

    // Add authorization header
    if k.cfg.K8s.InCluster {
        // For in-cluster, use the token from REST config
        if k.restConfig.BearerToken != "" {
            req.Header.Set("Authorization", "Bearer "+k.restConfig.BearerToken)
        }
    } else {
        // For external config, use the configured token
        if k.cfg.K8s.Token != "" {
            req.Header.Set("Authorization", "Bearer "+k.cfg.K8s.Token)
        }
    }

    // Add timeout to the request context
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    req = req.WithContext(ctx)

    resp, err := k.httpClient.Do(req)
    if err != nil {
        return false
    }
    defer resp.Body.Close()

    // Read response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return false
    }

    bodyStr := strings.TrimSpace(string(body))
    
    // Check if the API server is ready
    if resp.StatusCode == 200 && bodyStr == "ok" {
        return true
    }

    // Log details only when unhealthy (for debugging)
    log.Printf("K8s /readyz response for %s: status=%d, API server not ready", k.cfg.NodeID, resp.StatusCode)
    
    // Check for specific failure indicators
    lines := strings.Split(bodyStr, "\n")
    failedChecks := []string{}
    for _, line := range lines {
        if strings.HasPrefix(line, "[-]") {
            failedChecks = append(failedChecks, strings.TrimPrefix(line, "[-]"))
        }
    }
    
    if len(failedChecks) > 0 {
        log.Printf("K8s failed readiness checks: %v", failedChecks)
    }
    
    return false
}

func (k *K8sHealthChecker) IsHealthy() bool {
    if k == nil {
        return false
    }
    
    k.mu.RLock()
    defer k.mu.RUnlock()
    return k.healthy
}

func (k *K8sHealthChecker) GetHealthChangeChan() <-chan bool {
    if k == nil {
        return nil
    }
    return k.healthCh
}
