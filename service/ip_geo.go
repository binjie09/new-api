package service

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type IpInfo struct {
	Country   string `json:"country,omitempty"`
	Region    string `json:"region,omitempty"`
	City      string `json:"city,omitempty"`
	Isp       string `json:"isp,omitempty"`
	IsPrivate bool   `json:"is_private"`
	Display   string `json:"display,omitempty"`
}

type ipInfoCacheEntry struct {
	info      *IpInfo
	expiresAt time.Time
}

var (
	ipInfoCache   = make(map[string]*ipInfoCacheEntry)
	ipInfoCacheMu sync.RWMutex
)

const ipInfoCacheTTL = 10 * time.Minute

var ipGeoHttpClient = &http.Client{
	Timeout: 5 * time.Second,
}

// GetIpInfo returns attribution info for the given IP string.
// Private/loopback addresses return a minimal result without any remote lookup.
// Public addresses are looked up via ip-api.com (free, no key needed) and cached.
func GetIpInfo(ipStr string) *IpInfo {
	if ipStr == "" {
		return nil
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}

	// Private / loopback / link-local: no external lookup
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || common.IsPrivateIP(ip) {
		return &IpInfo{
			IsPrivate: true,
			Display:   "Private IP",
		}
	}

	// Check cache
	ipInfoCacheMu.RLock()
	if entry, ok := ipInfoCache[ipStr]; ok && time.Now().Before(entry.expiresAt) {
		ipInfoCacheMu.RUnlock()
		return entry.info
	}
	ipInfoCacheMu.RUnlock()

	// Perform lookup
	info := lookupIpInfo(ipStr)

	// Cache result
	ipInfoCacheMu.Lock()
	ipInfoCache[ipStr] = &ipInfoCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(ipInfoCacheTTL),
	}
	ipInfoCacheMu.Unlock()

	return info
}

// lookupIpInfo calls ip-api.com to resolve IP attribution.
// ip-api.com free tier: 45 requests/minute, HTTP only, JSON output.
func lookupIpInfo(ipStr string) *IpInfo {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,regionName,city,isp&lang=zh-CN", ipStr)

	resp, err := ipGeoHttpClient.Get(url)
	if err != nil {
		return &IpInfo{IsPrivate: false, Display: ipStr}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &IpInfo{IsPrivate: false, Display: ipStr}
	}

	var result struct {
		Status  string `json:"status"`
		Country string `json:"country"`
		Region  string `json:"regionName"`
		City    string `json:"city"`
		Isp     string `json:"isp"`
	}
	if err := common.Unmarshal(body, &result); err != nil || result.Status != "success" {
		return &IpInfo{IsPrivate: false, Display: ipStr}
	}

	info := &IpInfo{
		Country:   result.Country,
		Region:    result.Region,
		City:      result.City,
		Isp:       result.Isp,
		IsPrivate: false,
	}

	// Build display string
	parts := make([]string, 0, 3)
	if result.Country != "" {
		parts = append(parts, result.Country)
	}
	if result.Region != "" && result.Region != result.City {
		parts = append(parts, result.Region)
	}
	if result.City != "" {
		parts = append(parts, result.City)
	}
	display := ""
	for i, p := range parts {
		if i > 0 {
			display += " / "
		}
		display += p
	}
	if result.Isp != "" {
		if display != "" {
			display += " ("
		}
		display += result.Isp
		if len(parts) > 0 {
			display += ")"
		}
	}
	info.Display = display

	return info
}
