package promreceiver

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func Test_ScrapeBigFile(t *testing.T) {
	b, err := os.ReadFile("metrics.txt")
	require.NoError(t, err)
	s := newScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
	m, err := s.readFromResponse(b, "text/plain")
	require.NoError(t, err)
	fmt.Println(m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	mms := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < mms.Len(); i++ {
		fmt.Println(mms.At(i).Name())
	}
}
