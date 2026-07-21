package controller

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"streamingplayer/internal/service"
)

type TorrentController struct {
	torrentService service.TorrentService
}

func NewTorrentController(torrentService service.TorrentService) *TorrentController {
	return &TorrentController{torrentService: torrentService}
}

type DownloadTorrentInput struct {
	MagnetURL string `json:"magnet_url" binding:"required"`
}

type ScanTorrentURLInput struct {
	URL string `json:"url" binding:"required"`
}

func logApiPing(c *gin.Context, details string) {
	msg := fmt.Sprintf("command [transmission] : API pinged - %s %s", c.Request.Method, c.Request.URL.Path)
	if details != "" {
		msg += fmt.Sprintf(" (%s)", details)
	}
	slog.Info(msg)
	fmt.Println(msg)
}

// DownloadTorrent godoc
// @Summary Start downloading a video from a magnet link
// @Description Adds a magnet URI to the torrent client and begins background download
// @Tags torrent
// @Accept json
// @Produce json
// @Param input body DownloadTorrentInput true "Magnet URL"
// @Success 200 {object} map[string]interface{} "Created media record"
// @Router /torrent/download [post]
func (ctrl *TorrentController) DownloadTorrent(c *gin.Context) {
	var input DownloadTorrentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logApiPing(c, "invalid magnet_url input")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: magnet_url is required"})
		return
	}

	logApiPing(c, fmt.Sprintf("magnet: %s", input.MagnetURL))
	media, err := ctrl.torrentService.AddMagnet(c.Request.Context(), input.MagnetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": media})
}

// ListActiveTorrents godoc
// @Summary List all active torrent downloads
// @Description Returns the progress and status of all currently downloading torrents
// @Tags torrent
// @Produce json
// @Success 200 {object} map[string]interface{} "List of active torrent statuses"
// @Router /torrent/status [get]
func (ctrl *TorrentController) ListActiveTorrents(c *gin.Context) {
	logApiPing(c, "")
	statuses := ctrl.torrentService.ListActive()
	if statuses == nil {
		statuses = []service.TorrentStatus{}
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

// GetTorrentStatus godoc
// @Summary Get status of a specific torrent download
// @Description Returns detailed progress info for a given media ID's torrent download
// @Tags torrent
// @Produce json
// @Param id path string true "Media ID"
// @Success 200 {object} map[string]interface{} "Torrent status"
// @Failure 404 {object} map[string]interface{} "Torrent not found"
// @Router /torrent/status/{id} [get]
func (ctrl *TorrentController) GetTorrentStatus(c *gin.Context) {
	id := c.Param("id")
	logApiPing(c, fmt.Sprintf("id: %s", id))
	status, err := ctrl.torrentService.GetStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// CancelTorrent godoc
// @Summary Cancel and clean up a torrent download
// @Description Stops the torrent download, removes partially downloaded files, and marks the media as cancelled
// @Tags torrent
// @Produce json
// @Param id path string true "Media ID"
// @Success 200 {object} map[string]interface{} "Cancellation result"
// @Failure 400 {object} map[string]interface{} "No active torrent"
// @Router /torrent/cancel/{id} [post]
func (ctrl *TorrentController) CancelTorrent(c *gin.Context) {
	id := c.Param("id")
	logApiPing(c, fmt.Sprintf("cancel id: %s", id))
	if err := ctrl.torrentService.CancelTorrent(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": true})
}

// ScanTorrentURL godoc
// @Summary Scan HTML URL for magnet links
// @Description Fetches the HTML of page URL and returns detected magnet links
// @Tags torrent
// @Accept json
// @Produce json
// @Param input body ScanTorrentURLInput true "Page URL"
// @Success 200 {object} map[string]interface{} "List of TorrentTarget options"
// @Router /torrent/scan-url [post]
func (ctrl *TorrentController) ScanTorrentURL(c *gin.Context) {
	var input ScanTorrentURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logApiPing(c, "invalid url input")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: url is required"})
		return
	}

	logApiPing(c, fmt.Sprintf("scan url: %s", input.URL))
	targets, err := ctrl.torrentService.ScanHTML(c.Request.Context(), input.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": targets})
}

