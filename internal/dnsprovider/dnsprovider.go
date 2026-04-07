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

package dnsprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	hwdns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	dnsMdl "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	hwIam "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/iam/v3"
	iamMdl "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/iam/v3/model"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// HuaweicloudProvider is an implementation of Provider for HuaweiCloud DNS.
type HuaweicloudProvider struct {
	provider.BaseProvider
	domainFilter      *endpoint.DomainFilter
	zoneIDFilter      provider.ZoneIDFilter
	dnsClient         HuaweiCloudDNSAPI
	dryRun            bool
	privateZone       bool
	zoneMatchParent   bool
	config            *HuaweiCloudConfig
	vpcID             string
	tokenFile         string
	expirationSeconds int64
	expirationTime    int64
}

// HuaweiCloudConfig contains configuration to create a new HuaweiCloud provider.
type HuaweiCloudConfig struct {
	AccessKey     string `json:"accessKey"`
	SecretKey     string `json:"secretKey"`
	SecurityToken string `json:"securityToken"`
	Region        string `json:"region"`
	VpcID         string `json:"vpcId"`
	ProjectID     string `json:"projectId"`
	IdpID         string `json:"idpId"`
}

// RecordListGroup groups a DNS zone with its associated record sets.
type RecordListGroup struct {
	domain  dnsMdl.PrivateZoneResp
	records []dnsMdl.ListRecordSets
}

// NewHuaweiCloudProvider initializes a new HuaweiCloud based Provider.
func NewHuaweiCloudProvider(domainFilter *endpoint.DomainFilter, zoneIDFilter provider.ZoneIDFilter, configFile string, zoneType string, dryRun bool, tokenFile string, zoneMatchParent bool, expirationSeconds int64) (*HuaweicloudProvider, error) {
	cfg, err := parseConfig(configFile)
	if err != nil {
		return nil, err
	}

	var hcClient *core.HcHttpClient
	dnsRegion := region.NewRegion(cfg.Region, fmt.Sprintf("https://dns.%s.myhuaweicloud.com", cfg.Region))

	if tokenFile != "" {
		scopedToken, err := getScopedTokenByIdpToken(tokenFile, cfg)
		if err != nil {
			return nil, err
		}
		tokenAuth := hwIam.NewIamCredentialsBuilder().WithXAuthToken(scopedToken).Build()
		hcClient, err = hwdns.DnsClientBuilder().
			WithRegion(dnsRegion).
			WithCredential(tokenAuth).
			WithCredentialsType("v3.IamCredentials").
			WithHttpConfig(config.DefaultHttpConfig()).
			SafeBuild()
		if err != nil {
			return nil, fmt.Errorf("building HuaweiCloud DNS client with IDP token: %w", err)
		}
	} else {
		basicAuth, err := basic.NewCredentialsBuilder().
			WithAk(cfg.AccessKey).
			WithSk(cfg.SecretKey).
			WithSecurityToken(cfg.SecurityToken).
			SafeBuild()
		if err != nil {
			return nil, fmt.Errorf("building basic auth credentials: %w", err)
		}
		hcClient, err = hwdns.DnsClientBuilder().
			WithRegion(dnsRegion).
			WithCredential(basicAuth).
			WithHttpConfig(config.DefaultHttpConfig()).
			SafeBuild()
		if err != nil {
			return nil, fmt.Errorf("building HuaweiCloud DNS client: %w", err)
		}
	}

	return &HuaweicloudProvider{
		domainFilter:      domainFilter,
		zoneIDFilter:      zoneIDFilter,
		dnsClient:         hwdns.NewDnsClient(hcClient),
		vpcID:             cfg.VpcID,
		privateZone:       zoneType == "private",
		dryRun:            dryRun,
		zoneMatchParent:   zoneMatchParent,
		config:            cfg,
		tokenFile:         tokenFile,
		expirationSeconds: expirationSeconds,
	}, nil
}

// refreshToken refreshes the expired token when using IDP authentication.
func (p *HuaweicloudProvider) refreshToken() error {
	if p.tokenFile == "" {
		slog.Debug("using static credentials")
		return nil
	}

	currentTime := time.Now().Unix()
	if currentTime < p.expirationTime && p.expirationTime != 0 {
		return nil
	}
	p.expirationTime = currentTime + p.expirationSeconds

	slog.Debug("refreshing IDP token")
	scopedToken, err := getScopedTokenByIdpToken(p.tokenFile, p.config)
	if err != nil {
		return err
	}

	tokenAuth := hwIam.NewIamCredentialsBuilder().WithXAuthToken(scopedToken).Build()
	dnsRegion := region.NewRegion(p.config.Region, fmt.Sprintf("https://dns.%s.myhuaweicloud.com", p.config.Region))
	httpClient, err := hwdns.DnsClientBuilder().
		WithRegion(dnsRegion).
		WithCredential(tokenAuth).
		WithCredentialsType("v3.IamCredentials").
		WithHttpConfig(config.DefaultHttpConfig()).
		SafeBuild()
	if err != nil {
		return fmt.Errorf("rebuilding HuaweiCloud DNS client on token refresh: %w", err)
	}
	p.dnsClient = hwdns.NewDnsClient(httpClient)
	return nil
}

func parseConfig(configFile string) (*HuaweiCloudConfig, error) {
	if configFile == "" {
		return nil, fmt.Errorf("config file path is empty")
	}
	contents, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("reading HuaweiCloud config file %q: %w", configFile, err)
	}
	cfg := &HuaweiCloudConfig{}
	if err := json.Unmarshal(contents, cfg); err != nil {
		return nil, fmt.Errorf("parsing HuaweiCloud config file %q: %w", configFile, err)
	}
	return cfg, nil
}

func getScopedTokenByIdpToken(tokenFile string, cfg *HuaweiCloudConfig) (string, error) {
	basicAuth, err := basic.NewCredentialsBuilder().
		WithIdpId(cfg.IdpID).
		WithIdTokenFile(tokenFile).
		WithProjectId(cfg.ProjectID).
		SafeBuild()
	if err != nil {
		return "", fmt.Errorf("building IDP auth credentials: %w", err)
	}

	idToken, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("reading token file %q: %w", tokenFile, err)
	}

	iamRegion := region.NewRegion(cfg.Region, fmt.Sprintf("https://iam.%s.myhuaweicloud.com", cfg.Region))
	iamHttpClient, err := hwIam.IamClientBuilder().
		WithRegion(iamRegion).
		WithCredential(basicAuth).
		WithHttpConfig(config.DefaultHttpConfig()).
		SafeBuild()
	if err != nil {
		return "", fmt.Errorf("building IAM client: %w", err)
	}
	iamClient := hwIam.NewIamClient(iamHttpClient)

	// Obtain unscoped token.
	req := &iamMdl.CreateTokenWithIdTokenRequest{
		XIdpId: cfg.IdpID,
		Body:   &iamMdl.GetIdTokenRequestBody{Auth: &iamMdl.GetIdTokenAuthParams{}},
	}
	req.Body.Auth.IdToken = &iamMdl.GetIdTokenIdTokenBody{Id: string(idToken)}
	idTokenResponse, err := iamClient.CreateTokenWithIdToken(req)
	if err != nil {
		return "", fmt.Errorf("creating unscoped token with IDP token: %w", err)
	}
	unscopedToken := *idTokenResponse.XSubjectToken

	// Exchange for scoped token.
	scopedReq := &iamMdl.KeystoneCreateScopedTokenRequest{
		Body: &iamMdl.KeystoneCreateScopedTokenRequestBody{
			Auth: &iamMdl.ScopedTokenAuth{
				Scope:    &iamMdl.TokenSocpeOption{Project: &iamMdl.ScopeProjectOption{Id: &cfg.ProjectID}},
				Identity: &iamMdl.ScopedTokenIdentity{Methods: []string{"token"}, Token: &iamMdl.ScopedToken{Id: unscopedToken}},
			},
		},
	}
	scopedResp, err := iamClient.KeystoneCreateScopedToken(scopedReq)
	if err != nil {
		return "", fmt.Errorf("creating scoped token: %w", err)
	}
	return *scopedResp.XSubjectToken, nil
}

// Records returns the current DNS records.
func (p *HuaweicloudProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	slog.Info("retrieving HuaweiCloud DNS domain records")

	if err := p.refreshToken(); err != nil {
		return nil, err
	}

	zones, err := p.getZones()
	if err != nil {
		return nil, err
	}

	zoneRecordsGroup, err := p.getZoneRecordGroup(zones)
	if err != nil {
		return nil, err
	}

	var endpoints []*endpoint.Endpoint
	recordMap := groupRecords(zoneRecordsGroup)
	for _, recordList := range recordMap {
		for _, record := range recordList.records {
			name := cleanDomainName(*record.Name)
			endpoints = append(endpoints, endpoint.NewEndpointWithTTL(
				name, *record.Type, endpoint.TTL(*record.Ttl), *record.Records...,
			))
		}
	}
	return endpoints, nil
}

func groupRecords(zoneRecordsGroup map[string]RecordListGroup) map[string]RecordListGroup {
	endpointMap := make(map[string]RecordListGroup)
	for _, recordGroup := range zoneRecordsGroup {
		for _, record := range recordGroup.records {
			key := fmt.Sprintf("%s:%s", *record.Type, *record.Name)
			m := endpointMap[key]
			if m.records == nil {
				m = RecordListGroup{
					domain:  recordGroup.domain,
					records: make([]dnsMdl.ListRecordSets, 0),
				}
			}
			m.records = append(m.records, record)
			endpointMap[key] = m
		}
	}
	return endpointMap
}

func (p *HuaweicloudProvider) getZoneRecordGroup(zones []dnsMdl.PrivateZoneResp) (map[string]RecordListGroup, error) {
	recordListGroup := make(map[string]RecordListGroup)
	var length int
	for _, zone := range zones {
		recordSets, err := p.getZoneRecordList(*zone.Id)
		if err != nil {
			return nil, err
		}
		for i := range recordSets {
			if *recordSets[i].Type == "TXT" {
				records := make([]string, 0, len(*recordSets[i].Records))
				for _, record := range *recordSets[i].Records {
					records = append(records, unescapeTXTRecordValue(record))
				}
				recordSets[i].Records = &records
			}
		}
		recordListGroup[*zone.Id] = RecordListGroup{
			domain:  zone,
			records: recordSets,
		}
		length += len(recordSets)
	}
	slog.Info("found HuaweiCloud DNS records", "count", length)
	return recordListGroup, nil
}

func unescapeTXTRecordValue(value string) string {
	if strings.HasPrefix(value, "heritage=") {
		return fmt.Sprintf("%q", strings.ReplaceAll(value, ";", ","))
	}
	return value
}

func (p *HuaweicloudProvider) getZones() ([]dnsMdl.PrivateZoneResp, error) {
	var zones []dnsMdl.PrivateZoneResp

	req := &dnsMdl.ListPrivateZonesRequest{}
	req.Offset = int32Ptr(0)
	req.Limit = int32Ptr(50)
	totalCount := int32(50)
	if p.privateZone {
		req.Type = "private"
	}

	for *req.Offset < totalCount {
		resp, err := p.dnsClient.ListPrivateZones(req)
		if err != nil {
			return nil, fmt.Errorf("listing HuaweiCloud DNS zones: %w", err)
		}
		for _, zone := range *resp.Zones {
			if p.domainFilter.IsConfigured() {
				if !p.domainFilter.Match(*zone.Name) {
					if !p.zoneMatchParent || !p.domainFilter.MatchParent(*zone.Name) {
						continue
					}
				}
			}
			if p.zoneIDFilter.IsConfigured() && !p.zoneIDFilter.Match(*zone.Id) {
				continue
			}
			if p.privateZone && !p.matchVPC(*zone.Id) {
				continue
			}
			zones = append(zones, zone)
		}
		totalCount = *resp.Metadata.TotalCount
		req.Offset = int32Ptr(*req.Offset + int32(len(*resp.Zones)))
	}
	return zones, nil
}

func (p *HuaweicloudProvider) matchVPC(zoneID string) bool {
	if p.vpcID == "" {
		return true
	}
	resp, err := p.dnsClient.ShowPrivateZone(&dnsMdl.ShowPrivateZoneRequest{ZoneId: zoneID})
	if err != nil {
		return false
	}
	for _, vpc := range *resp.Routers {
		if vpc.RouterId == p.vpcID {
			return true
		}
	}
	return false
}

func (p *HuaweicloudProvider) getZoneRecordList(zoneID string) ([]dnsMdl.ListRecordSets, error) {
	req := &dnsMdl.ListRecordSetsByZoneRequest{
		ZoneId: zoneID,
		Offset: int32Ptr(0),
		Limit:  int32Ptr(50),
	}
	totalCount := int32(50)

	var recordList []dnsMdl.ListRecordSets
	for *req.Offset < totalCount {
		resp, err := p.dnsClient.ListRecordSetsByZone(req)
		if err != nil {
			return nil, fmt.Errorf("listing records for zone %s: %w", zoneID, err)
		}
		for _, recordSet := range *resp.Recordsets {
			if !provider.SupportedRecordType(*recordSet.Type) || *recordSet.Default {
				continue
			}
			recordList = append(recordList, recordSet)
		}
		totalCount = *resp.Metadata.TotalCount
		req.Offset = int32Ptr(*req.Offset + int32(len(*resp.Recordsets)))
	}
	return recordList, nil
}

func equalStringSlice(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	if (a == nil) != (b == nil) {
		return false
	}
	for i := range a {
		if strings.TrimSuffix(a[i], ".") != strings.TrimSuffix(b[i], ".") {
			return false
		}
	}
	return true
}

// jsonWrapper marshals an object to JSON for logging purposes.
func jsonWrapper(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return "json_format_error"
	}
	return string(data)
}

func cleanDomainName(domain string) string {
	return strings.TrimSuffix(domain, ".")
}

// ApplyChanges applies the given DNS changes.
func (p *HuaweicloudProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if !changes.HasChanges() {
		return nil
	}
	slog.Info("applying changes", "changes", jsonWrapper(changes))

	if err := p.refreshToken(); err != nil {
		return err
	}

	zones, err := p.getZones()
	if err != nil {
		return err
	}

	zoneNameIDMap := make(map[string][]string)
	if p.privateZone && p.vpcID == "" {
		for _, zone := range zones {
			zoneNameIDMap[*zone.Name] = append(zoneNameIDMap[*zone.Name], *zone.Id)
		}
	}

	zoneRecordsGroup, err := p.getZoneRecordGroup(zones)
	if err != nil {
		return err
	}

	// Skip private zones with duplicate names when no VPC filter is set.
	if p.privateZone && p.vpcID == "" {
		slog.Info("checking HuaweiCloud DNS private zone name conflicts")
		for name, ids := range zoneNameIDMap {
			if len(ids) > 1 {
				for _, id := range ids {
					delete(zoneRecordsGroup, id)
				}
				slog.Error("conflict: multiple zones with same name, skipping", "name", name)
			}
		}
	}

	zoneNameIDMapper := provider.ZoneIDName{}
	for _, group := range zoneRecordsGroup {
		if *group.domain.Id != "" {
			zoneNameIDMapper.Add(*group.domain.Id, cleanDomainName(*group.domain.Name))
		}
	}

	// Process deletes (Delete + UpdateOld).
	deleteChanges := append(changes.Delete, changes.UpdateOld...)
	deleteEndpoints := p.getDeleteRecordIDsMap(zoneNameIDMapper, deleteChanges, zoneRecordsGroup)
	var failedZones []string
	failedZones = append(failedZones, p.deleteRecords(deleteEndpoints)...)

	// Process creates (Create + UpdateNew).
	createEndpoints := make(map[string][]*endpoint.Endpoint)
	createChanges := append(changes.Create, changes.UpdateNew...)
	for _, change := range createChanges {
		zoneID, _ := zoneNameIDMapper.FindZone(cleanDomainName(change.DNSName))
		if zoneID == "" {
			slog.Info("no matching zone for creating record", "type", change.RecordType, "name", change.DNSName)
			continue
		}
		createEndpoints[zoneID] = append(createEndpoints[zoneID], change)
	}

	seen := make(map[string]struct{})
	for _, z := range failedZones {
		seen[z] = struct{}{}
	}
	for _, z := range p.createRecords(createEndpoints) {
		if _, ok := seen[z]; !ok {
			failedZones = append(failedZones, z)
		}
	}

	if len(failedZones) > 0 {
		return provider.NewSoftError(fmt.Errorf("failed to submit changes for zones: %v", failedZones))
	}
	return nil
}

func (p *HuaweicloudProvider) getDeleteRecordIDsMap(zoneNameIDMapper provider.ZoneIDName, changes []*endpoint.Endpoint, domainRecordsGroup map[string]RecordListGroup) map[string][]string {
	deleteEndpoints := make(map[string][]string)
	for _, change := range changes {
		zoneID, _ := zoneNameIDMapper.FindZone(cleanDomainName(change.DNSName))
		if zoneID == "" {
			continue
		}
		for _, record := range domainRecordsGroup[zoneID].records {
			if cleanDomainName(*record.Name) == cleanDomainName(change.DNSName) &&
				*record.Type == change.RecordType &&
				equalStringSlice(*record.Records, change.Targets) {
				deleteEndpoints[zoneID] = append(deleteEndpoints[zoneID], *record.Id)
			}
		}
	}
	return deleteEndpoints
}

func (p *HuaweicloudProvider) createRecords(endpointsMap map[string][]*endpoint.Endpoint) (failedZones []string) {
	for zoneID, endpoints := range endpointsMap {
		if err := p.createRecordByZoneID(zoneID, endpoints); err != nil {
			failedZones = append(failedZones, zoneID)
		}
	}
	return
}

func (p *HuaweicloudProvider) createRecordByZoneID(zoneID string, endpoints []*endpoint.Endpoint) error {
	for _, ep := range endpoints {
		req := &dnsMdl.CreateRecordSetRequest{
			ZoneId: zoneID,
			Body: &dnsMdl.CreateRecordSetRequestBody{
				Name:    ep.DNSName,
				Type:    ep.RecordType,
				Records: ep.Targets,
			},
		}
		if ep.RecordTTL.IsConfigured() {
			req.Body.Ttl = int32Ptr(int32(ep.RecordTTL))
		}
		if p.dryRun {
			slog.Info("dry run: create record", "type", ep.RecordType, "name", ep.DNSName, "targets", ep.Targets, "ttl", ep.RecordTTL)
			continue
		}
		response, err := p.dnsClient.CreateRecordSet(req)
		if err != nil {
			slog.Error("failed to create record", "name", ep.DNSName, "error", err)
			return err
		}
		slog.Info("created record", "type", ep.RecordType, "name", ep.DNSName, "targets", ep.Targets.String(), "ttl", ep.RecordTTL, "id", *response.Id)
	}
	return nil
}

func (p *HuaweicloudProvider) deleteRecords(recordIDsMap map[string][]string) (failedZones []string) {
	for zoneID, recordIDs := range recordIDsMap {
		if err := p.deleteRecordsByZoneID(zoneID, recordIDs); err != nil {
			failedZones = append(failedZones, zoneID)
		}
	}
	return
}

func (p *HuaweicloudProvider) deleteRecordsByZoneID(zoneID string, recordIDs []string) error {
	for _, recordID := range recordIDs {
		if p.dryRun {
			slog.Info("dry run: delete record", "recordId", recordID, "zoneId", zoneID)
			continue
		}
		response, err := p.dnsClient.DeleteRecordSet(&dnsMdl.DeleteRecordSetRequest{
			ZoneId:      zoneID,
			RecordsetId: recordID,
		})
		if err != nil {
			slog.Error("failed to delete record", "recordId", recordID, "error", err)
			return err
		}
		slog.Info("deleted record", "recordId", *response.Id, "zoneId", zoneID)
	}
	return nil
}

func int32Ptr(v int32) *int32 {
	return &v
}
