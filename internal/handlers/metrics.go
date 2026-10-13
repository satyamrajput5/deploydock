package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/satyamrajput5/deploydock/internal/metrics"
)

type MetricsHandler struct {
	collector *metrics.Collector
}

func NewMetricsHandler(collector *metrics.Collector) *MetricsHandler {
	return &MetricsHandler{collector: collector}
}

func (h *MetricsHandler) GetSystemStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.collector.GetOverview())
}

func (h *MetricsHandler) GetDeploymentStats(c *gin.Context) {
	deploymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID format"})
		return
	}

	metric, ok := h.collector.GetDeploymentMetric(deploymentID)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"deployment_id": deploymentID,
			"is_running":    false,
			"cpu_pct":       0.0,
			"mem_mb":        0.0,
			"mem_perc":      0.0,
			"mem_usage_raw": "0MB / 0MB",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployment_id": deploymentID,
		"container_id":  metric.ContainerID,
		"build_id":      metric.BuildID,
		"name":          metric.Name,
		"is_running":    true,
		"cpu_pct":       metric.CPUPct,
		"mem_mb":        metric.MemMB,
		"mem_perc":      metric.MemPerc,
		"mem_usage_raw": metric.MemUsageRaw,
		"updated_at":    metric.UpdatedAt,
	})
}
