package main

import (
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
}

type GeoIP struct {
	dbPath   string
	enabled  bool
	cityReader *geoip2.Reader
	ispReader  *geoip2.Reader
	mu       sync.RWMutex
	cache    *lruCache[string, *GeoLocation]
}

func NewGeoIP(dbPath string, enabled bool) *GeoIP {
	g := &GeoIP{
		dbPath:  dbPath,
		enabled: enabled,
		cache:   newLRUCache[string, *GeoLocation](10000),
	}
	if enabled {
		g.open()
		g.startWatcher()
	}
	return g
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

	g.mu.Lock()
	if g.cityReader != nil {
		g.cityReader.Close()
	}
	g.cityReader = reader
	if g.ispReader != nil {
		g.ispReader.Close()
	}
	g.ispReader = ispReader
	g.mu.Unlock()

	g.cache.Flush()

	logInfo(nil, "geoip: database loaded", "path", g.dbPath)
}

func (g *GeoIP) startWatcher() {
	go func() {
		dir := filepath.Dir(g.dbPath)
		for {
			w, err := fsnotify.NewWatcher()
			if err != nil {
				logError(nil, "geoip watcher create error", "error", err.Error())
				time.Sleep(30 * time.Second)
				continue
			}
			if err := w.Add(dir); err != nil {
				w.Close()
				logError(nil, "geoip watcher add error", "error", err.Error())
				time.Sleep(30 * time.Second)
				continue
			}
			for event := range w.Events {
				if event.Name == g.dbPath && event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					time.Sleep(500 * time.Millisecond)
					logInfo(nil, "geoip: database file changed, reloading")
					g.open()
				}
			}
			w.Close()
		}
	}()
}

func (g *GeoIP) Lookup(ipStr string) *GeoLocation {
	if !g.enabled {
		return nil
	}

	if loc, ok := g.cache.Get(ipStr); ok {
		return loc
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}

	g.mu.RLock()
	reader := g.cityReader
	ispReader := g.ispReader
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
		loc.City = record.City.Names["en"]
	}
	if record.Country.GeoNameID != 0 {
		loc.Country = record.Country.Names["en"]
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

	g.cache.Set(ipStr, loc)
	return loc
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
}

func ispDBPath(cityDBPath string) string {
	return strings.TrimSuffix(cityDBPath, "-City.mmdb") + "-ISP.mmdb"
}
