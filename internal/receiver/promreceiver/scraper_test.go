package promreceiver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
)

func Test_ReadSampleData(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "metrics_sample.txt"))
	require.NoError(t, err)
	set := receivertest.NewNopCreateSettings()
	set.Logger, _ = zap.NewDevelopment()
	s := newScraper(set, createDefaultConfig().(*Config))
	m, err := s.readFromResponse(b, "text/plain")
	require.NoError(t, err)
	require.Equal(t, 3, m.DataPointCount())
	metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	names := []string{metrics.At(0).Name(), metrics.At(1).Name(), metrics.At(2).Name()}
	require.Contains(t, names, "foo_bar_gc_cycles_total_gc_cycles_total")
	require.Contains(t, names, "foo_bar_gc_duration_seconds")
	require.Contains(t, names, "foo_bar_gc_heap_allocs_by_size_bytes_total")

	for i := 0; i < metrics.Len(); i++ {
		switch metrics.At(i).Name() {
		case "foo_bar_gc_cycles_total_gc_cycles_total":
			require.Equal(t, 1, metrics.At(i).Sum().DataPoints().Len())
		case "foo_bar_gc_duration_seconds":
			require.Equal(t, 1, metrics.At(i).Summary().DataPoints().Len())
		case "foo_bar_gc_heap_allocs_by_size_bytes_total":
			require.Equal(t, 1, metrics.At(i).Histogram().DataPoints().Len())
			require.Equal(t, 12, metrics.At(i).Histogram().DataPoints().At(0).BucketCounts().Len())
		default:
			require.Fail(t, "should not happen, unknown metric "+metrics.At(i).Name())
		}
	}

}
