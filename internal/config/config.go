// Package config: layered INI, same files and keys as v0.1 so a box
// migrating from the Python build keeps its configuration. Go stdlib
// has no INI reader; the subset parser below handles [sections],
// key=value, and # ; comments — nothing else, on purpose.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	SystemConf      = "/etc/stoker/config.ini"
	SystemPluginDir = "/usr/share/stoker/plugins"
	EtcPluginDir    = "/etc/stoker/plugins"
)

type Config struct {
	PluginDirs         []string
	JournalLines       int
	ConfirmDestructive bool
	AllowUserPlugins   bool
}

func xdgConfig() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return x
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func parseINI(path string, into map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line[1 : len(line)-1])
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			into[section+"."+strings.ToLower(strings.TrimSpace(k))] =
				strings.TrimSpace(v)
		}
	}
}

func truthy(s string, def bool) bool {
	switch strings.ToLower(s) {
	case "1", "yes", "true", "on":
		return true
	case "0", "no", "false", "off":
		return false
	}
	return def
}

func Load() Config {
	kv := map[string]string{}
	parseINI(SystemConf, kv)
	parseINI(filepath.Join(xdgConfig(), "stoker", "config.ini"), kv)

	cfg := Config{JournalLines: 400, ConfirmDestructive: true}
	if v, ok := kv["stoker.journal_lines"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.JournalLines = n
		}
	}
	cfg.ConfirmDestructive = truthy(kv["stoker.confirm_destructive"], true)
	cfg.AllowUserPlugins = truthy(kv["stoker.allow_user_plugins"], false)

	cfg.PluginDirs = []string{SystemPluginDir, EtcPluginDir}
	if extra := kv["plugins.dirs"]; extra != "" {
		for _, p := range strings.Split(extra, ":") {
			if p = strings.TrimSpace(p); p != "" {
				cfg.PluginDirs = append(cfg.PluginDirs, p)
			}
		}
	}
	if cfg.AllowUserPlugins {
		cfg.PluginDirs = append(cfg.PluginDirs,
			filepath.Join(xdgConfig(), "stoker", "plugins"))
	}
	if env := os.Getenv("STOKER_PLUGIN_DIR"); env != "" {
		cfg.PluginDirs = append(cfg.PluginDirs, env)
	}
	return cfg
}
