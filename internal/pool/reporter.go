package pool

import (
	"context"
	"sync"
	"time"
	"xiaozhi-esp32-server-golang/internal/components/http"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

// StatsReporter resource pool stats reporter
type StatsReporter struct {
	client  *http.ManagerClient
	enabled bool
}

var (
	globalReporter *StatsReporter
	reporterOnce   sync.Once
)

// GetStatsReporter gets global stats reporter (singleton)
func GetStatsReporter() *StatsReporter {
	reporterOnce.Do(func() {
		// get manager backend URL, priority from environment variable, if env var not exist then from config
		baseURL := util.GetBackendURL()
		if baseURL == "" {
			baseURL = "http://localhost:8080" // default values
		}

		// check if enable reporting
		enabled := viper.GetBool("pool_stats.report_enabled")
		if !enabled {
			// default enable
			enabled = true
		}

		// create HTTP client
		managerClient := http.NewManagerClient(http.ManagerClientConfig{
			BaseURL:    baseURL,
			AuthToken:  util.GetManagerAuthToken(),
			Timeout:    5 * time.Second,
			MaxRetries: 2,
		})

		globalReporter = &StatsReporter{
			client:  managerClient,
			enabled: enabled,
		}

		log.Infof("resource pool stats reporter already initialized, backend_url=%s, enabled=%v", baseURL, enabled)
	})
	return globalReporter
}

// StartReporting starts stats reporting (every 5 seconds report once)
func (r *StatsReporter) StartReporting(ctx context.Context) {
	if !r.enabled {
		log.Info("resource pool stats reporting already disabled")
		return
	}

	// report interval (5 seconds)
	interval := viper.GetDuration("pool_stats.report_interval")
	if interval == 0 {
		interval = 5 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		//log.Infof("resource pool stats reporting already started, reports every %v", interval)

		for {
			select {
			case <-ctx.Done():
				log.Debugf("resource pool stats reporting already stopped")
				return
			case <-ticker.C:
				r.reportStats(ctx)
			}
		}
	}()
}

// reportStats reports stats data
func (r *StatsReporter) reportStats(ctx context.Context) {
	// get stats data
	stats := GetStats()

	// if no data, skip reporting
	if len(stats) == 0 {
		//log.Debugf("currently no active resource pool, skip reporting")
		return
	}

	// build request body
	requestBody := map[string]interface{}{
		"stats": stats,
	}

	// send report request
	err := r.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   "/api/internal/pool/stats",
		Body:   requestBody,
	})

	if err != nil {
		log.Warnf("resource pool stats reporting failed: %v", err)
	} else {
		//log.Debugf("resource pool stats reporting successful, resource pool count: %d", len(stats))
	}
}

// StartStatsReporter starts global stats reporter (convenience function)
func StartStatsReporter(ctx context.Context) {
	reporter := GetStatsReporter()
	reporter.StartReporting(ctx)
}
