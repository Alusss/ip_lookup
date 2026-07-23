package main

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/oschwald/geoip2-golang"
)

type GeoLocation struct {
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
	ISP     string `json:"isp,omitempty"`
	ASN     string `json:"asn,omitempty"`
}

type GeoIP struct {
	dbPath        string
	asnDbPath     string
	enabled       bool
	cityReader    *geoip2.Reader
	ispReader     *geoip2.Reader
	asnReader     *geoip2.Reader
	watcherStarted bool
	mu            sync.RWMutex
	cache         *lruCache[string, *GeoLocation]
}

func NewGeoIP(dbPath, asnDbPath string, enabled bool) *GeoIP {
	g := &GeoIP{
		dbPath:    dbPath,
		asnDbPath: asnDbPath,
		enabled:   enabled,
		cache:     newLRUCache[string, *GeoLocation](10000),
	}
	if enabled {
		g.open()
		g.mu.Lock()
		g.watcherStarted = true
		g.mu.Unlock()
		g.startWatcher()
	}
	return g
}

// Configure applies a runtime config change (hot reload). It re-opens the
// databases when enabled/path changes, or closes them when disabled. The fsnotify
// watcher is started lazily on first enable.
func (g *GeoIP) Configure(enabled bool, dbPath, asnDbPath string) {
	g.mu.Lock()
	changed := g.enabled != enabled || g.dbPath != dbPath || g.asnDbPath != asnDbPath
	if !changed {
		g.mu.Unlock()
		return
	}
	wasEnabled := g.enabled
	g.enabled = enabled
	g.dbPath = dbPath
	g.asnDbPath = asnDbPath
	needWatcher := enabled && !g.watcherStarted
	if needWatcher {
		g.watcherStarted = true
	}
	g.mu.Unlock()

	if enabled {
		logInfo(nil, "geoip: reconfiguring", "path", dbPath, "asn", asnDbPath)
		g.open()
		if needWatcher {
			g.startWatcher()
		}
	} else if wasEnabled {
		logInfo(nil, "geoip: disabling")
		g.closeReaders()
		g.cache.Flush()
	}
}

func (g *GeoIP) closeReaders() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cityReader != nil {
		g.cityReader.Close()
		g.cityReader = nil
	}
	if g.ispReader != nil {
		g.ispReader.Close()
		g.ispReader = nil
	}
	if g.asnReader != nil {
		g.asnReader.Close()
		g.asnReader = nil
	}
}

func (g *GeoIP) open() {
	reader, err := geoip2.Open(g.dbPath)
	if err != nil {
		logWarn(nil, "geoip: cannot open database, geo features disabled", "path", g.dbPath, "error", err.Error())
		return
	}

	ispPath := ispDBPath(g.dbPath)
	ispReader, err := geoip2.Open(ispPath)
	if err != nil {
		ispReader = nil
	}

	var asnReader *geoip2.Reader
	if g.asnDbPath != "" {
		if ar, err := geoip2.Open(g.asnDbPath); err == nil {
			asnReader = ar
		}
	}

	g.mu.Lock()
	if g.cityReader != nil {
		g.cityReader.Close()
	}
	g.cityReader = reader
	if g.ispReader != nil {
		g.ispReader.Close()
	}
	g.ispReader = ispReader
	if g.asnReader != nil {
		g.asnReader.Close()
	}
	g.asnReader = asnReader
	g.mu.Unlock()

	g.cache.Flush()

	logInfo(nil, "geoip: database loaded", "path", g.dbPath, "asn", g.asnDbPath)
}

func (g *GeoIP) startWatcher() {
	go func() {
		for {
			g.mu.RLock()
			dir := filepath.Dir(g.dbPath)
			g.mu.RUnlock()

			w, err := fsnotify.NewWatcher()
			if err != nil {
				logError(nil, "geoip watcher create error", "error", err.Error())
				time.Sleep(30 * time.Second)
				continue
			}
			if err := w.Add(dir); err != nil {
				w.Close()
				logError(nil, "geoip watcher add error", "dir", dir, "error", err.Error())
				time.Sleep(30 * time.Second)
				continue
			}
			for event := range w.Events {
				g.mu.RLock()
				dbPath := g.dbPath
				asnDbPath := g.asnDbPath
				en := g.enabled
				g.mu.RUnlock()
				if (event.Name == dbPath || event.Name == asnDbPath) && event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					time.Sleep(500 * time.Millisecond)
					if en {
						logInfo(nil, "geoip: database file changed, reloading", "file", event.Name)
						g.open()
					}
				}
			}
			w.Close()
		}
	}()
}

func (g *GeoIP) Lookup(ipStr string, lang string) *GeoLocation {
	if !g.enabled {
		return nil
	}

	cacheKey := lang + "|" + ipStr
	if loc, ok := g.cache.Get(cacheKey); ok {
		return loc
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}

	g.mu.RLock()
	reader := g.cityReader
	ispReader := g.ispReader
	asnReader := g.asnReader
	g.mu.RUnlock()

	if reader == nil {
		return nil
	}

	record, err := reader.City(ip)
	if err != nil {
		return nil
	}

	loc := &GeoLocation{}

	if record.City.GeoNameID != 0 {
		loc.City = pickName(record.City.Names, lang)
	}
	if record.Country.GeoNameID != 0 {
		loc.Country = pickName(record.Country.Names, lang)
	}
	if loc.City == "" {
		loc.City = loc.Country
	}

	if ispReader != nil {
		ispRecord, err := ispReader.ISP(ip)
		if err == nil && ispRecord.ISP != "" {
			loc.ISP = ispRecord.ISP
		}
	}

	if asnReader != nil {
		if asnRecord, err := asnReader.ASN(ip); err == nil {
			loc.ASN = formatASN(asnRecord.AutonomousSystemNumber, asnRecord.AutonomousSystemOrganization)
		}
	}

	g.cache.Set(cacheKey, loc)
	return loc
}

// pickName returns the localized name for the given language, falling back to
// English (MaxMind always ships en) when the requested language is absent.
func pickName(names map[string]string, lang string) string {
	if lang == "zh" {
		if v, ok := names["zh-CN"]; ok && v != "" {
			return v
		}
	}
	return names["en"]
}

func formatASN(num uint, org string) string {
	if num == 0 && org == "" {
		return ""
	}
	if num == 0 {
		return org
	}
	if org == "" {
		return fmt.Sprintf("AS%d", num)
	}
	return fmt.Sprintf("AS%d %s", num, org)
}

func (g *GeoIP) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cityReader != nil {
		g.cityReader.Close()
	}
	if g.ispReader != nil {
		g.ispReader.Close()
	}
	if g.asnReader != nil {
		g.asnReader.Close()
	}
}

func ispDBPath(cityDBPath string) string {
	return strings.TrimSuffix(cityDBPath, "-City.mmdb") + "-ISP.mmdb"
}
