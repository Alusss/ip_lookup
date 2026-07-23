package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type IPExtractor struct {
	cfCIDRs      []*net.IPNet
	cfCIDRsMu    sync.RWMutex
	cfCIDRPath   string
	proxyCIDRs   []*net.IPNet
	proxyCIDRsMu sync.RWMutex
}

func NewIPExtractor(cfCidrPath string, reloadInterval time.Duration) *IPExtractor {
	e := &IPExtractor{
		cfCIDRPath: cfCidrPath,
	}
	e.loadCfCIDRs()
	e.startCfCidrWatcher()
	e.startCfCidrFallbackTimer(reloadInterval)
	return e
}

// startCfCidrFallbackTimer periodically re-reads the CF CIDR file as a safety
// net in case an fsnotify event is missed (e.g. certain filesystems, atomic
// replaces across mounts). The primary reload mechanism remains fsnotify.
func (e *IPExtractor) startCfCidrFallbackTimer(interval time.Duration) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			e.loadCfCIDRs()
		}
	}()
}

func (e *IPExtractor) loadCfCIDRs() {
	data, err := os.ReadFile(e.cfCIDRPath)
	if err != nil {
		logWarn(nil, "cannot read CF CIDR file, using empty list", "path", e.cfCIDRPath, "error", err.Error())
		return
	}

	var cidrs []*net.IPNet
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		_, cidr, err := net.ParseCIDR(line)
		if err != nil {
			logWarn(nil, "invalid CF CIDR", "cidr", line, "error", err.Error())
			continue
		}
		cidrs = append(cidrs, cidr)
	}

	e.cfCIDRsMu.Lock()
	e.cfCIDRs = cidrs
	e.cfCIDRsMu.Unlock()

	logInfo(nil, "loaded CF CIDRs", "count", len(cidrs), "path", e.cfCIDRPath)
}

func (e *IPExtractor) startCfCidrWatcher() {
	go func() {
		dir := filepath.Dir(e.cfCIDRPath)

		for {
			w, err := fsnotify.NewWatcher()
			if err != nil {
				logError(nil, "cf cidr watcher create error", "error", err.Error())
				time.Sleep(10 * time.Second)
				continue
			}

			if err := w.Add(dir); err != nil {
				w.Close()
				logError(nil, "cf cidr watcher add error", "error", err.Error())
				time.Sleep(10 * time.Second)
				continue
			}

			for event := range w.Events {
				if event.Name == e.cfCIDRPath && event.Op&(fsnotify.Write|fsnotify.Rename|fsnotify.Create) != 0 {
					time.Sleep(200 * time.Millisecond)
					e.loadCfCIDRs()
				}
			}

			w.Close()
		}
	}()
}

func (e *IPExtractor) UpdateProxyCIDRs(cidrStrs []string) {
	var cidrs []*net.IPNet
	for _, s := range cidrStrs {
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			logWarn(nil, "invalid proxy CIDR", "cidr", s, "error", err.Error())
			continue
		}
		cidrs = append(cidrs, cidr)
	}

	e.proxyCIDRsMu.Lock()
	e.proxyCIDRs = cidrs
	e.proxyCIDRsMu.Unlock()
}

func (e *IPExtractor) isCfIP(ip net.IP) bool {
	e.cfCIDRsMu.RLock()
	defer e.cfCIDRsMu.RUnlock()

	for _, cidr := range e.cfCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func (e *IPExtractor) isProxyIP(ip net.IP) bool {
	e.proxyCIDRsMu.RLock()
	defer e.proxyCIDRsMu.RUnlock()

	for _, cidr := range e.proxyCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// IsSourceTrusted reports whether the direct TCP peer of r is a Cloudflare edge
// IP or a configured trusted proxy. Used by the cf_only guard to reject direct
// connections that bypass the CDN / reverse proxy.
func (e *IPExtractor) IsSourceTrusted(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return e.isCfIP(ip) || e.isProxyIP(ip)
}

func (e *IPExtractor) RealIP(r *http.Request) (net.IP, error) {
	hostPort := r.RemoteAddr
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
	}

	remoteIP := net.ParseIP(host)
	if remoteIP == nil {
		return nil, fmt.Errorf("invalid remote addr: %s", hostPort)
	}

	if e.isCfIP(remoteIP) {
		cfIP := r.Header.Get("CF-Connecting-IP")
		if cfIP != "" {
			parsed := net.ParseIP(cfIP)
			if parsed != nil {
				return parsed, nil
			}
		}
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			if ip := parseXFF(xff); ip != nil {
				return ip, nil
			}
		}
		return remoteIP, nil
	}

	if e.isProxyIP(remoteIP) {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			if ip := parseXFF(xff); ip != nil {
				return ip, nil
			}
		}
		return remoteIP, nil
	}

	return remoteIP, nil
}

func parseXFF(xff string) net.IP {
	parts := strings.Split(xff, ",")
	if len(parts) == 0 {
		return nil
	}
	ipStr := strings.TrimSpace(parts[len(parts)-1])
	ip := net.ParseIP(ipStr)
	return ip
}
