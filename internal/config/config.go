package config

import (
    "gopkg.in/yaml.v2"
    "log"
    "os"
)

type K8sConfig struct {
    Enabled     bool   `yaml:"enabled"`
    APIServer   string `yaml:"api_server"`
    Token       string `yaml:"token"`
    CACert      string `yaml:"ca_cert"`
    InCluster   bool   `yaml:"in_cluster"`
}

type Config struct {
    K8s              K8sConfig `yaml:"k8s"`
    NodeID           string    `yaml:"node_id"`
    Priority         int       `yaml:"priority"`
    Interface        string    `yaml:"interface"`
    VIP              string    `yaml:"vip"`
    Peers            []string  `yaml:"peers"`
    Port             int       `yaml:"port"`
    HeartbeatInterval int      `yaml:"heartbeat_interval"`
    ElectionTimeout   int      `yaml:"election_timeout"`
    TLSCert          string    `yaml:"tls_cert"`
    TLSKey           string    `yaml:"tls_key"`
}

func LoadConfig(path string) *Config {
    data, err := os.ReadFile(path)
    if err != nil {
        log.Fatalf("Failed to read config: %v", err)
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        log.Fatalf("Failed to parse config: %v", err)
    }
    return &cfg
}
