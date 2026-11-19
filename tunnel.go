package kernel

import (
	"fmt"
	"os/exec"
	"strings"
)

type TunnelConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	LocalIP     string `json:"local_ip"`
	RemoteIP    string `json:"remote_ip"`
	TunnelCIDR  string `json:"tunnel_cidr"`
	IPv6        bool   `json:"ipv6"`
	MTU         int    `json:"mtu"`
	TTL         int    `json:"ttl"`
	RateLimit   int    `json:"rate_limit_mbps"`
	SpoofMode   string `json:"spoof_mode"`
	SpoofSNI    string `json:"spoof_sni"`
	Enabled     bool   `json:"enabled"`
}

func BuildCommands(cfg TunnelConfig) []string {
	ifName := fmt.Sprintf("gld-%s", cfg.ID)
	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 1420
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 64
	}

	var cmds []string
	switch strings.ToLower(cfg.Protocol) {
	case "gre", "ip6gre":
		mode := "gre"
		if cfg.IPv6 || strings.ToLower(cfg.Protocol) == "ip6gre" {
			mode = "ip6gre"
		}
		cmds = append(cmds, fmt.Sprintf("ip tunnel add %s mode %s remote %s local %s ttl %d", ifName, mode, cfg.RemoteIP, cfg.LocalIP, ttl))
	case "gretap":
		cmds = append(cmds, fmt.Sprintf("ip link add %s type gretap remote %s local %s ttl %d", ifName, cfg.RemoteIP, cfg.LocalIP, ttl))
	case "ipip", "ip6ip6":
		mode := "ipip"
		if cfg.IPv6 || strings.ToLower(cfg.Protocol) == "ip6ip6" {
			mode = "ip6ip6"
		}
		cmds = append(cmds, fmt.Sprintf("ip tunnel add %s mode %s remote %s local %s ttl %d", ifName, mode, cfg.RemoteIP, cfg.LocalIP, ttl))
	case "sit", "6to4":
		cmds = append(cmds, fmt.Sprintf("ip tunnel add %s mode sit remote %s local %s ttl %d", ifName, cfg.RemoteIP, cfg.LocalIP, ttl))
	case "vxlan":
		cmds = append(cmds, fmt.Sprintf("ip link add %s type vxlan id 100 remote %s local %s dstport 4789", ifName, cfg.RemoteIP, cfg.LocalIP))
	case "geneve":
		cmds = append(cmds, fmt.Sprintf("ip link add %s type geneve id 100 remote %s dstport 6081", ifName, cfg.RemoteIP))
	case "wireguard", "amneziawg":
		cmds = append(cmds, fmt.Sprintf("ip link add dev %s type wireguard", ifName))
	case "xfrm-i":
		cmds = append(cmds, fmt.Sprintf("ip link add %s type xfrm if_id 42", ifName))
	case "ipsec-vti":
		cmds = append(cmds, fmt.Sprintf("ip tunnel add %s mode vti local %s remote %s key 42", ifName, cfg.LocalIP, cfg.RemoteIP))
	default:
		cmds = append(cmds, fmt.Sprintf("# Custom/Spoof protocol (%s) managed by GOLDEN daemon", cfg.Protocol))
	}

	if !strings.HasPrefix(strings.ToLower(cfg.Protocol), "spoof") {
		cmds = append(cmds,
			fmt.Sprintf("ip link set %s mtu %d up", ifName, mtu),
			fmt.Sprintf("ip addr add %s dev %s", cfg.TunnelCIDR, ifName),
		)
		if cfg.RateLimit > 0 {
			cmds = append(cmds, fmt.Sprintf("tc qdisc replace dev %s root tbf rate %dmbit burst 32kbit latency 40ms", ifName, cfg.RateLimit))
		}
	}
	return cmds
}

func ApplyProfile(profile string) error {
	sysctls := map[string]string{
		"net.core.default_qdisc":          "fq",
		"net.ipv4.tcp_congestion_control": "bbr",
		"net.ipv4.ip_forward":             "1",
		"net.ipv6.conf.all.forwarding":    "1",
	}
	if profile == "max" {
		sysctls["net.core.rmem_max"] = "67108864"
		sysctls["net.core.wmem_max"] = "67108864"
		sysctls["net.core.netdev_max_backlog"] = "250000"
	} else if profile == "lean" {
		sysctls["net.ipv4.tcp_low_latency"] = "1"
		sysctls["net.ipv4.tcp_nodelay"] = "1"
	}

	for k, v := range sysctls {
		_ = exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", k, v)).Run()
	}
	return nil
}
