package controllers

import (
	"net/http"
	"time"
	"xiaozhi/manager/backend/storage"

	"github.com/gin-gonic/gin"
)

// PoolStatsController resource pool statistics controller
type PoolStatsController struct {
	storage *storage.PoolStatsStorage
}

// NewPoolStatsController create resource pool statistics controller
func NewPoolStatsController() *PoolStatsController {
	return &PoolStatsController{
		storage: storage.GetPoolStatsStorage(),
	}
}

// ReportPoolStats receive statistics data reported by main service (internal interface, no auth required)
func (c *PoolStatsController) ReportPoolStats(ctx *gin.Context) {
	var request struct {
		Stats map[string]interface{} `json:"stats" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	// Save statistics data
	c.storage.AddStats(request.Stats)

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "Statistics data reported successfully",
		"timestamp": time.Now().Unix(),
	})
}

// GetPoolStats get resource pool statistics data (admin interface)
func (c *PoolStatsController) GetPoolStats(ctx *gin.Context) {
	// Get query parameters
	queryType := ctx.DefaultQuery("type", "latest") // latest, all, range

	switch queryType {
	case "latest":
		// Get latest data
		latest := c.storage.GetLatestStats()
		if latest == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"data":    nil,
				"message": "No statistics data available",
			})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"data": latest,
		})

	case "all":
		// Get all data (last 24 hours)
		allStats := c.storage.GetAllStats()
		ctx.JSON(http.StatusOK, gin.H{
			"data":  allStats,
			"count": len(allStats),
		})

	case "range":
		// Get data by time range
		startStr := ctx.Query("start")
		endStr := ctx.Query("end")

		if startStr == "" || endStr == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Time range parameters start and end cannot be empty"})
			return
		}

		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Start time format error, please use RFC3339 format"})
			return
		}

		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "End time format error, please use RFC3339 format"})
			return
		}

		stats := c.storage.GetStatsByTimeRange(start, end)
		ctx.JSON(http.StatusOK, gin.H{
			"data":  stats,
			"count": len(stats),
		})

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query type, supported: latest, all, range"})
	}
}

// GetPoolStatsSummary get statistics summary information
func (c *PoolStatsController) GetPoolStatsSummary(ctx *gin.Context) {
	latest := c.storage.GetLatestStats()

	summary := gin.H{
		"total_records":    0,
		"storage_duration": "Only latest data saved",
		"oldest_timestamp": nil,
		"newest_timestamp": nil,
	}

	if latest != nil {
		summary["total_records"] = 1
		summary["newest_timestamp"] = latest.Timestamp
		summary["oldest_timestamp"] = latest.Timestamp
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": summary,
	})
}
