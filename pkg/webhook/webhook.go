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

package webhook

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const (
	mediaTypeFormat        = "application/external.dns.webhook+json;"
	contentTypeHeader      = "Content-Type"
	contentTypePlaintext   = "text/plain"
	acceptHeader           = "Accept"
	varyHeader             = "Vary"
	supportedMediaVersions = "1"
)

var mediaTypeVersion1 = mediaTypeVersion("1")

type mediaType string

func mediaTypeVersion(v string) mediaType {
	return mediaType(mediaTypeFormat + "version=" + v)
}

func (m mediaType) Is(headerValue string) bool {
	return string(m) == headerValue
}

func checkAndGetMediaTypeHeaderValue(value string) (string, error) {
	for _, v := range strings.Split(supportedMediaVersions, ",") {
		if mediaTypeVersion(v).Is(value) {
			return v, nil
		}
	}
	var supported []string
	for _, v := range strings.Split(supportedMediaVersions, ",") {
		supported = append(supported, string(mediaTypeVersion(v)))
	}
	return "", fmt.Errorf("unsupported media type version: %q. Supported: %s", value, strings.Join(supported, ", "))
}

// Webhook wraps a provider.Provider with HTTP handlers for the ExternalDNS webhook protocol.
type Webhook struct {
	provider provider.Provider
}

// New creates a new Webhook instance.
func New(provider provider.Provider) *Webhook {
	return &Webhook{provider: provider}
}

func (p *Webhook) contentTypeHeaderCheck(w http.ResponseWriter, r *http.Request) error {
	return p.headerCheck(true, w, r)
}

func (p *Webhook) acceptHeaderCheck(w http.ResponseWriter, r *http.Request) error {
	return p.headerCheck(false, w, r)
}

func (p *Webhook) headerCheck(isContentType bool, w http.ResponseWriter, r *http.Request) error {
	var header string
	if isContentType {
		header = r.Header.Get(contentTypeHeader)
	} else {
		header = r.Header.Get(acceptHeader)
	}

	headerName := "accept header"
	if isContentType {
		headerName = "content type"
	}

	if header == "" {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusNotAcceptable)
		err := fmt.Errorf("client must provide a %s", headerName)
		if _, writeErr := fmt.Fprint(w, err.Error()); writeErr != nil {
			requestLog(r).Error("error writing response", "error", writeErr)
		}
		return err
	}

	if _, err := checkAndGetMediaTypeHeaderValue(header); err != nil {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusUnsupportedMediaType)
		err := fmt.Errorf("client must provide a valid versioned media type in the %s: %w", headerName, err)
		if _, writeErr := fmt.Fprint(w, err.Error()); writeErr != nil {
			requestLog(r).Error("error writing response", "error", writeErr)
		}
		return err
	}
	return nil
}

// Records handles GET /records.
func (p *Webhook) Records(w http.ResponseWriter, r *http.Request) {
	if err := p.acceptHeaderCheck(w, r); err != nil {
		requestLog(r).Error("accept header check failed", "error", err)
		return
	}
	requestLog(r).Debug("requesting records")

	records, err := p.provider.Records(r.Context())
	if err != nil {
		requestLog(r).Error("error getting records", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	requestLog(r).Debug("returning records", "count", len(records))
	w.Header().Set(contentTypeHeader, string(mediaTypeVersion1))
	w.Header().Set(varyHeader, contentTypeHeader)
	if err := json.NewEncoder(w).Encode(records); err != nil {
		requestLog(r).Error("error encoding records", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// ApplyChanges handles POST /records.
func (p *Webhook) ApplyChanges(w http.ResponseWriter, r *http.Request) {
	if err := p.contentTypeHeaderCheck(w, r); err != nil {
		requestLog(r).Error("content type header check failed", "error", err)
		return
	}

	var changes plan.Changes
	if err := json.NewDecoder(r.Body).Decode(&changes); err != nil {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusBadRequest)
		errMsg := fmt.Sprintf("error decoding changes: %s", err.Error())
		if _, writeErr := fmt.Fprint(w, errMsg); writeErr != nil {
			requestLog(r).Error("error writing response", "error", writeErr)
		}
		requestLog(r).Info(errMsg, "error", err)
		return
	}

	requestLog(r).Debug("applying changes",
		"create", len(changes.Create), "updateOld", len(changes.UpdateOld),
		"updateNew", len(changes.UpdateNew), "delete", len(changes.Delete))

	if err := p.provider.ApplyChanges(r.Context(), &changes); err != nil {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdjustEndpoints handles POST /adjustendpoints.
func (p *Webhook) AdjustEndpoints(w http.ResponseWriter, r *http.Request) {
	if err := p.contentTypeHeaderCheck(w, r); err != nil {
		requestLog(r).Error("content type header check failed", "error", err)
		return
	}
	if err := p.acceptHeaderCheck(w, r); err != nil {
		requestLog(r).Error("accept header check failed", "error", err)
		return
	}

	var pve []*endpoint.Endpoint
	if err := json.NewDecoder(r.Body).Decode(&pve); err != nil {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusBadRequest)
		errMsg := fmt.Sprintf("failed to decode request body: %v", err)
		requestLog(r).Info(errMsg)
		if _, writeErr := fmt.Fprint(w, errMsg); writeErr != nil {
			requestLog(r).Error("error writing response", "error", writeErr)
		}
		return
	}

	slog.Debug("adjusting endpoints", "count", len(pve))
	pve, err := p.provider.AdjustEndpoints(pve)
	if err != nil {
		slog.Error("failed to adjust endpoints", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	out, _ := json.Marshal(&pve)
	slog.Debug("returning adjusted endpoints", "count", len(pve))
	w.Header().Set(contentTypeHeader, string(mediaTypeVersion1))
	w.Header().Set(varyHeader, contentTypeHeader)
	if _, writeErr := fmt.Fprint(w, string(out)); writeErr != nil {
		requestLog(r).Error("error writing response", "error", writeErr)
	}
}

// Negotiate handles GET / — returns the domain filter.
func (p *Webhook) Negotiate(w http.ResponseWriter, r *http.Request) {
	if err := p.acceptHeaderCheck(w, r); err != nil {
		requestLog(r).Error("accept header check failed", "error", err)
		return
	}
	b, err := json.Marshal(p.provider.GetDomainFilter())
	if err != nil {
		requestLog(r).Error("failed to marshal domain filter", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentTypeHeader, string(mediaTypeVersion1))
	if _, writeErr := w.Write(b); writeErr != nil {
		requestLog(r).Error("error writing response", "error", writeErr)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func requestLog(r *http.Request) *slog.Logger {
	return slog.With("method", r.Method, "path", r.URL.Path)
}
