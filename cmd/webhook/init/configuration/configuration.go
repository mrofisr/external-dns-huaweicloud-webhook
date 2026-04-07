/*
Copyright 2023 Huaweicloud

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configuration

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds configuration parsed from environment variables.
type Config struct {
	ServerHost           string
	ServerPort           int
	ServerReadTimeout    time.Duration
	ServerWriteTimeout   time.Duration
	DomainFilter         []string
	ExcludeDomains       []string
	ZoneNameFilter       []string
	ZoneIDFilter         []string
	RegexDomainFilter    string
	RegexDomainExclusion string
	DryRun               bool
	ConfigFile           string
	ZoneType             string
	TokenFile            string
	ZoneMatchParent      bool
	ExpirationSeconds    int64
}

// Init reads configuration from environment variables.
func Init() Config {
	cfg := Config{
		ServerHost:           envStr("SERVER_HOST", "localhost"),
		ServerPort:           envInt("SERVER_PORT", 8888),
		ServerReadTimeout:    envDuration("SERVER_READ_TIMEOUT", 0),
		ServerWriteTimeout:   envDuration("SERVER_WRITE_TIMEOUT", 0),
		DomainFilter:         envSlice("DOMAIN_FILTER"),
		ExcludeDomains:       envSlice("EXCLUDE_DOMAINS"),
		ZoneNameFilter:       envSlice("ZONE_NAME_FILTER"),
		ZoneIDFilter:         envSlice("ZONE_ID_FILTER"),
		RegexDomainFilter:    envStr("REGEXP_DOMAIN_FILTER", ""),
		RegexDomainExclusion: envStr("REGEXP_DOMAIN_FILTER_EXCLUSION", ""),
		DryRun:               envBool("DRY_RUN", false),
		ConfigFile:           envStr("CONFIG_FILE", "/etc/kubernetes/huawei-cloud.json"),
		ZoneType:             envStr("ZONE_TYPE", "public"),
		TokenFile:            envStr("TOKEN_FILE", ""),
		ZoneMatchParent:      envBool("ZONE_MATCH_PARENT", false),
		ExpirationSeconds:    envInt64("EXPIRATION_SECONDS", 7200),
	}
	return cfg
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		slog.Warn("invalid int env var", "key", key, "value", v)
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
		slog.Warn("invalid int64 env var", "key", key, "value", v)
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		slog.Warn("invalid bool env var", "key", key, "value", v)
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("invalid duration env var", "key", key, "value", v)
	}
	return def
}

func envSlice(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
