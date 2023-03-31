// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promreceiver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/prometheus/prometheus/model/textparse"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type scraper struct {
	client   *http.Client
	settings component.TelemetrySettings
	cfg      *Config
}

func newScraper(
	settings receiver.CreateSettings,
	cfg *Config,
) *scraper {
	e := &scraper{
		settings: settings.TelemetrySettings,
		cfg:      cfg,
	}

	return e
}

func (s *scraper) start(_ context.Context, host component.Host) error {
	var err error
	s.client, err = s.cfg.ToClient(host, s.settings)
	return err
}

func (s *scraper) scrape(context.Context) (pmetric.Metrics, error) {
	resp, err := s.client.Get(s.cfg.Endpoint)
	m := pmetric.NewMetrics()
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return m, fmt.Errorf("Expecting a 200 response, got: %d with %s", resp.StatusCode, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	return s.readFromResponse(b, resp.Header.Get("Content-Type"))
}

func (s *scraper) readFromResponse(b []byte, contentType string) (pmetric.Metrics, error) {
	m := pmetric.NewMetrics()
	metricFamilies, err := ParseMetricFamilies(b, contentType, time.Now())
	if err != nil {
		return m, err
	}
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	for _, family := range metricFamilies {
		newMetric := sm.Metrics().AppendEmpty()
		newMetric.SetName(family.GetName())
		newMetric.SetUnit(family.GetUnit())
		switch family.Type {
		case textparse.MetricTypeCounter:
			sum := newMetric.SetEmptySum()
			sum.SetIsMonotonic(true)
			for _, fm := range family.GetMetric() {
				dp := sum.DataPoints().AppendEmpty()
				dp.SetDoubleValue(fm.GetCounter().GetValue())
				for _, l := range fm.GetLabel() {
					dp.Attributes().PutStr(l.Name, l.Value)
				}
			}
		case textparse.MetricTypeGauge:
			gauge := newMetric.SetEmptyGauge()
			for _, fm := range family.Metric {
				dp := gauge.DataPoints().AppendEmpty()
				dp.SetDoubleValue(fm.GetGauge().GetValue())
				for _, l := range fm.GetLabel() {
					dp.Attributes().PutStr(l.Name, l.Value)
				}
			}
		case textparse.MetricTypeHistogram:
			histogram := newMetric.SetEmptyHistogram()
			for _, fm := range family.Metric {
				dp := histogram.DataPoints().AppendEmpty()
				for _, b := range fm.GetHistogram().GetBucket() {
					dp.BucketCounts().Append(b.GetCumulativeCount())
					dp.ExplicitBounds().Append(b.GetUpperBound())
				}
				dp.SetSum(fm.GetHistogram().GetSampleSum())
				dp.SetCount(fm.GetHistogram().GetSampleCount())
				for _, l := range fm.GetLabel() {
					dp.Attributes().PutStr(l.Name, l.Value)
				}
			}
		case textparse.MetricTypeSummary:
			sum := newMetric.SetEmptySummary()
			for _, fm := range family.Metric {
				dp := sum.DataPoints().AppendEmpty()
				for _, q := range fm.GetSummary().GetQuantile() {
					newQ := dp.QuantileValues().AppendEmpty()
					newQ.SetValue(q.GetValue())
					newQ.SetQuantile(q.GetQuantile())
				}
				dp.SetSum(fm.GetSummary().GetSampleSum())
				dp.SetCount(fm.GetSummary().GetSampleCount())
				for _, l := range fm.GetLabel() {
					dp.Attributes().PutStr(l.Name, l.Value)
				}
			}
		default:
			s.settings.Logger.Warn("No mapping present for metric family", zap.Any("family", family.Type))
		}
	}
	return m, nil
}
