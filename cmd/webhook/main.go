/*
Copyright 2023 GleSYS AB

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

package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"

	"github.com/setoru/external-dns-huaweicloud-webhook/cmd/webhook/init/configuration"
	"github.com/setoru/external-dns-huaweicloud-webhook/cmd/webhook/init/logging"
	"github.com/setoru/external-dns-huaweicloud-webhook/cmd/webhook/init/server"
	"github.com/setoru/external-dns-huaweicloud-webhook/internal/dnsprovider"
	"github.com/setoru/external-dns-huaweicloud-webhook/pkg/webhook"
)

func main() {
	logging.Init()
	config := configuration.Init()

	var domainFilter *endpoint.DomainFilter
	createMsg := "Creating HuaweiCloud provider with "

	if config.RegexDomainFilter != "" {
		createMsg += fmt.Sprintf("regexp domain filter: %q", config.RegexDomainFilter)
		if config.RegexDomainExclusion != "" {
			createMsg += fmt.Sprintf(", exclusion: %q", config.RegexDomainExclusion)
		}
		domainFilter = endpoint.NewRegexDomainFilter(
			regexp.MustCompile(config.RegexDomainFilter),
			regexp.MustCompile(config.RegexDomainExclusion),
		)
	} else {
		if len(config.DomainFilter) > 0 {
			createMsg += fmt.Sprintf("domain filter: %s", strings.Join(config.DomainFilter, ","))
		}
		if len(config.ExcludeDomains) > 0 {
			createMsg += fmt.Sprintf(", exclude: %s", strings.Join(config.ExcludeDomains, ","))
		}
		domainFilter = endpoint.NewDomainFilterWithExclusions(config.DomainFilter, config.ExcludeDomains)
	}
	slog.Info(createMsg)

	zoneIDFilter := provider.NewZoneIDFilter(config.ZoneIDFilter)
	p, err := dnsprovider.NewHuaweiCloudProvider(
		domainFilter, zoneIDFilter,
		config.ConfigFile, config.ZoneType, config.DryRun,
		config.TokenFile, config.ZoneMatchParent, config.ExpirationSeconds,
	)
	if err != nil {
		slog.Error("failed to initialize DNS provider", "error", err)
		os.Exit(1)
	}

	srv := server.Init(config, webhook.New(p))
	server.ShutdownGracefully(srv)
}
