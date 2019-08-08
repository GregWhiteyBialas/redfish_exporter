package collector

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	gofish "github.com/stmcginnis/gofish/school"
	gofishcommon "github.com/stmcginnis/gofish/school/common"
	redfish "github.com/stmcginnis/gofish/school/redfish"
)

// Metric name parts.
const (
	// Exporter namespace.
	namespace = "redfish"
	// Subsystem(s).
	exporter = "exporter"
	// Math constant for picoseconds to seconds.
	picoSeconds = 1e12
)

// Metric descriptors.
var (
	totalScrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		nil, nil,
	)
)

// Exporter collects redfish metrics. It implements prometheus.Collector.
type RedfishCollector struct {
	redfishClient  *gofish.ApiClient
	collectors     map[string]prometheus.Collector
	redfishUp      prometheus.Gauge
	redfishUpValue bool
}

func NewRedfishCollector(host string, username string, password string) *RedfishCollector {
	redfishClient, redfishUpValue := newRedfishClient(host, username, password)
	chassisCollector := NewChassisCollector(namespace, redfishClient)
	systemCollector := NewSystemCollector(namespace, redfishClient)

	return &RedfishCollector{
		redfishClient: redfishClient,
		collectors:    map[string]prometheus.Collector{"chassis": chassisCollector, "system": systemCollector},
		redfishUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "",
				Name:      "up",
				Help:      "redfish up",
			},
		),
		redfishUpValue: redfishUpValue,
	}
}

// Describe implements prometheus.Collector.
func (r *RedfishCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range r.collectors {
		collector.Describe(ch)
	}

}

// Collect implements prometheus.Collector.
func (r *RedfishCollector) Collect(ch chan<- prometheus.Metric) {
	scrapeTime := time.Now()
	if r.redfishUpValue {
		r.redfishUp.Set(1)
		ch <- r.redfishUp
		for _, collector := range r.collectors {
			collector.Collect(ch)
		}
	} else {
		r.redfishUp.Set(0)
		ch <- r.redfishUp
	}
	ch <- prometheus.MustNewConstMetric(totalScrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), )
}

func newRedfishClient(host string, username string, password string) (*gofish.ApiClient, bool) {

	url := fmt.Sprintf("https://%s", host)

	// skip ssl verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{Transport: tr}
	// Create a new instance of gofish client
	redfishClient, err := gofish.APIClient(url, httpClient)
	if err != nil {
		log.Infof("Errors occours when creating redfish client: %s", err)
		return redfishClient, false
	}

	service, err := gofish.ServiceRoot(redfishClient)
	if err != nil {
		log.Infof("Errors occours when Getting Services: %s", err)
		return redfishClient, false
	}

	// Generates a authenticated session
	auth, err := service.CreateSession(username, password)
	if err != nil {
		log.Infof("Errors occours when creating sessions: %s", err)
		return redfishClient, false
	}

	// Assign the token back to our gofish client
	redfishClient.Token = auth.Token
	log.Infof("Scraping target %s", host)
	return redfishClient, true
}

func parseCommonStatusHealth(status gofishcommon.Health) (float64, bool) {
	if bytes.Equal([]byte(status), []byte("OK")) {
		return float64(1), true
	} else if bytes.Equal([]byte(status), []byte("Warning")) {
		return float64(2), true
	} else if bytes.Equal([]byte(status), []byte("Critical")) {
		return float64(3), true
	}
	return float64(0), false
}

func parseCommonStatusState(status gofishcommon.State) (float64, bool) {
	if bytes.Equal([]byte(status), []byte("Enabled")) {
		return float64(1), true
	} else if bytes.Equal([]byte(status), []byte("Disabled")) {
		return float64(2), true
	} else if bytes.Equal([]byte(status), []byte("StandbyOffinline")) {
		return float64(3), true
	} else if bytes.Equal([]byte(status), []byte("StandbySpare")) {
		return float64(4), true
	} else if bytes.Equal([]byte(status), []byte("InTest")) {
		return float64(5), true
	} else if bytes.Equal([]byte(status), []byte("Starting")) {
		return float64(6), true
	} else if bytes.Equal([]byte(status), []byte("Absent")) {
		return float64(7), true
	} else if bytes.Equal([]byte(status), []byte("UnavailableOffline")) {
		return float64(8), true
	} else if bytes.Equal([]byte(status), []byte("Deferring")) {
		return float64(9), true
	} else if bytes.Equal([]byte(status), []byte("Quiesced")) {
		return float64(10), true
	} else if bytes.Equal([]byte(status), []byte("Updating")) {
		return float64(11), true
	}
	return float64(0), false
}

func parseSystemPowerState(status redfish.PowerState) (float64, bool) {
	if bytes.Equal([]byte(status), []byte("On")) {
		return float64(1), true
	} else if bytes.Equal([]byte(status), []byte("Off")) {
		return float64(2), true
	} else if bytes.Equal([]byte(status), []byte("PoweringOn")) {
		return float64(3), true
	} else if bytes.Equal([]byte(status), []byte("PoweringOff")) {
		return float64(4), true
	}
	return float64(0), false
}

func parseLinkStatus(status redfish.LinkStatus) (float64, bool) {
	if bytes.Equal([]byte(status), []byte("LinkUp")) {
		return float64(1), true
	} else if bytes.Equal([]byte(status), []byte("NoLink")) {
		return float64(2), true
	} else if bytes.Equal([]byte(status), []byte("LinkDown")) {
		return float64(3), true
	}
	return float64(0), false
}

func boolToFloat64(data bool) float64 {

	if data {
		return float64(1)
	} else {
		return float64(0)
	}
}
