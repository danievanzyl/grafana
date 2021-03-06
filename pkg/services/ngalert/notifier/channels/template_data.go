package channels

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	gokit_log "github.com/go-kit/kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/ngalert/logging"
)

type ExtendedAlert struct {
	Status       string      `json:"status"`
	Labels       template.KV `json:"labels"`
	Annotations  template.KV `json:"annotations"`
	StartsAt     time.Time   `json:"startsAt"`
	EndsAt       time.Time   `json:"endsAt"`
	GeneratorURL string      `json:"generatorURL"`
	Fingerprint  string      `json:"fingerprint"`
	SilenceURL   string      `json:"silenceURL"`
	DashboardURL string      `json:"dashboardURL"`
	PanelURL     string      `json:"panelURL"`
}

type ExtendedAlerts []ExtendedAlert

type ExtendedData struct {
	Receiver string         `json:"receiver"`
	Status   string         `json:"status"`
	Alerts   ExtendedAlerts `json:"alerts"`

	GroupLabels       template.KV `json:"groupLabels"`
	CommonLabels      template.KV `json:"commonLabels"`
	CommonAnnotations template.KV `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

func removePrivateItems(kv template.KV) template.KV {
	for key := range kv {
		if strings.HasPrefix(key, "__") && strings.HasSuffix(key, "__") {
			kv = kv.Remove([]string{key})
		}
	}
	return kv
}

func extendAlert(alert template.Alert, externalURL string) (*ExtendedAlert, error) {
	extended := ExtendedAlert{
		Status:       alert.Status,
		Labels:       alert.Labels,
		Annotations:  alert.Annotations,
		StartsAt:     alert.StartsAt,
		EndsAt:       alert.EndsAt,
		GeneratorURL: alert.GeneratorURL,
		Fingerprint:  alert.Fingerprint,
	}

	// fill in some grafana-specific urls
	if len(externalURL) > 0 {
		u, err := url.Parse(externalURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse external URL: %w", err)
		}
		externalPath := u.Path
		dashboardUid := alert.Annotations["__dashboardUid__"]
		if len(dashboardUid) > 0 {
			u.Path = path.Join(externalPath, "/d/", dashboardUid)
			extended.DashboardURL = u.String()
			panelId := alert.Annotations["__panelId__"]
			if len(panelId) > 0 {
				u.RawQuery = "viewPanel=" + panelId
				extended.PanelURL = u.String()
			}
		}

		matchers := make([]string, 0)
		for key, value := range alert.Labels {
			if !(strings.HasPrefix(key, "__") && strings.HasSuffix(key, "__")) {
				matchers = append(matchers, key+"="+value)
			}
		}
		sort.Strings(matchers)
		u.Path = path.Join(externalPath, "/alerting/silence/new")
		u.RawQuery = "alertmanager=grafana&matchers=" + url.QueryEscape(strings.Join(matchers, ","))
		extended.SilenceURL = u.String()
	}

	// remove "private" annotations & labels so they don't show up in the template
	extended.Annotations = removePrivateItems(extended.Annotations)
	extended.Labels = removePrivateItems(extended.Labels)

	return &extended, nil
}

func ExtendData(data *template.Data) (*ExtendedData, error) {
	alerts := []ExtendedAlert{}

	for _, alert := range data.Alerts {
		extendedAlert, err := extendAlert(alert, data.ExternalURL)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, *extendedAlert)
	}

	extended := &ExtendedData{
		Receiver:          data.Receiver,
		Status:            data.Status,
		Alerts:            alerts,
		GroupLabels:       data.GroupLabels,
		CommonLabels:      removePrivateItems(data.CommonLabels),
		CommonAnnotations: removePrivateItems(data.CommonAnnotations),

		ExternalURL: data.ExternalURL,
	}
	return extended, nil
}

func TmplText(ctx context.Context, tmpl *template.Template, alerts []*types.Alert, l log.Logger, tmplErr *error) (func(string) string, *ExtendedData, error) {
	promTmplData := notify.GetTemplateData(ctx, tmpl, alerts, gokit_log.NewLogfmtLogger(logging.NewWrapper(l)))
	data, err := ExtendData(promTmplData)
	if err != nil {
		return nil, nil, err
	}

	return func(name string) (s string) {
		if *tmplErr != nil {
			return
		}
		s, *tmplErr = tmpl.ExecuteTextString(name, data)
		return s
	}, data, nil
}

// Firing returns the subset of alerts that are firing.
func (as ExtendedAlerts) Firing() []ExtendedAlert {
	res := []ExtendedAlert{}
	for _, a := range as {
		if a.Status == string(model.AlertFiring) {
			res = append(res, a)
		}
	}
	return res
}

// Resolved returns the subset of alerts that are resolved.
func (as ExtendedAlerts) Resolved() []ExtendedAlert {
	res := []ExtendedAlert{}
	for _, a := range as {
		if a.Status == string(model.AlertResolved) {
			res = append(res, a)
		}
	}
	return res
}
