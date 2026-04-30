package cloud

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yingshulu/famfun/internal/model"
)

var allowedUploadExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
}

type CloudServer struct {
	router       *gin.Engine
	homeRegistry HomeRegistry
	streamProxy  StreamProxyService
	cache        SegmentCache
	videoStore   *VideoStore
	distDir      string
	jwtKey       []byte
	tokenExpiry  time.Duration
}

func NewCloudServer(registry HomeRegistry, proxy StreamProxyService, cache SegmentCache, videoStore *VideoStore, distDir string) *CloudServer {
	key, err := generateSigningKey()
	if err != nil {
		panic("failed to generate JWT signing key: " + err.Error())
	}

	s := &CloudServer{
		router:       gin.Default(),
		homeRegistry: registry,
		streamProxy:  proxy,
		cache:        cache,
		videoStore:   videoStore,
		distDir:      distDir,
		jwtKey:       key,
		tokenExpiry:  30 * time.Minute,
	}
	s.setupRoutes()
	return s
}

func (s *CloudServer) setupRoutes() {
	auth := s.router.Group("/auth")
	auth.POST("/login", s.handleLogin)
	auth.POST("/logout", s.handleLogout)

	s.router.GET("/api/avatar/:username", s.handleGetAvatar)

	api := s.router.Group("/api")
	api.Use(s.authMiddleware())
	api.GET("/videos", s.handleListVideos)
	api.GET("/thumbnail/:homeID/:videoID", s.handleGetThumbnail)
	api.GET("/stream/:homeID/:videoID/:resource", s.handleStreamResource)
	api.GET("/cache-stats", s.handleCacheStats)
	api.POST("/videos/:videoID/play", s.handlePlayVideo)
	api.GET("/videos/:videoID/comments", s.handleGetComments)
	api.POST("/videos/:videoID/comments", s.memberOrAdmin(), s.handleAddComment)
	api.GET("/me", s.handleCurrentUser)
	api.PUT("/videos/:videoID/info", s.adminOnly(), s.handleUpdateVideoInfo)
	api.POST("/upload", s.memberOrAdmin(), s.handleUploadVideo)
	api.GET("/homes", s.memberOrAdmin(), s.handleListHomes)
	api.GET("/home-status", s.handleHomeStatus)
	api.POST("/homes/:homeID/scan", s.adminOnly(), s.handleScanHome)

	admin := api.Group("/admin")
	admin.Use(s.adminOnly())
	admin.POST("/users", s.handleCreateUser)
	admin.GET("/users", s.handleListUsers)
	admin.DELETE("/users/:userID", s.handleDeleteUser)
	admin.PUT("/users/:userID/role", s.handleUpdateUserRole)
	admin.PUT("/users/:userID/password", s.handleUpdateUserPassword)

	s.router.NoRoute(s.handleStaticFiles)
}

func (s *CloudServer) Run(addr string) error {
	return s.router.Run(addr)
}

func canAccess(role, visibility string) bool {
	switch role {
	case "admin":
		return true
	case "member":
		return visibility == "member" || visibility == "guest"
	default:
		return visibility == "guest"
	}
}

func (s *CloudServer) handleListVideos(c *gin.Context) {
	role := c.GetString("role")

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	page, total := s.homeRegistry.ListVideosPage(offset, limit, func(v *model.Video) bool {
		return canAccess(role, v.Visibility)
	})

	videoIDs := make([]string, len(page))
	for i, v := range page {
		videoIDs[i] = v.ID
	}

	pageStats, err := s.videoStore.GetVideoStats(videoIDs)
	if err != nil {
		pageStats = map[string]VideoStats{}
	}

	result := make([]videoJSON, len(page))
	for i, v := range page {
		result[i] = videoToJSON(v)
		if vs, ok := pageStats[v.ID]; ok {
			result[i].PlayCount = vs.PlayCount
			result[i].CommentCount = vs.CommentCount
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":   result,
		"total":    total,
		"has_more": offset+limit < total,
	})
}

func (s *CloudServer) checkVideoAccess(c *gin.Context, videoID string) bool {
	v, ok := s.homeRegistry.FindVideoByID(videoID)
	if !ok {
		return true
	}
	if !canAccess(c.GetString("role"), v.Visibility) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return false
	}
	return true
}

func (s *CloudServer) handleGetThumbnail(c *gin.Context) {
	homeID := c.Param("homeID")
	videoID := c.Param("videoID")

	if !s.checkVideoAccess(c, videoID) {
		return
	}

	data, err := s.streamProxy.GetThumbnail(homeID, videoID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "image/png", data)
}

func (s *CloudServer) handleStreamResource(c *gin.Context) {
	homeID := c.Param("homeID")
	videoID := c.Param("videoID")
	resource := c.Param("resource")

	if !s.checkVideoAccess(c, videoID) {
		return
	}

	if resource == "index.m3u8" {
		s.servePlaylist(c, homeID, videoID)
		return
	}

	s.serveSegment(c, homeID, videoID, resource)
}

func (s *CloudServer) servePlaylist(c *gin.Context, homeID, videoID string) {
	data, err := s.streamProxy.GetPlaylist(homeID, videoID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", data)
}

func (s *CloudServer) serveSegment(c *gin.Context, homeID, videoID, segmentName string) {
	data, err := s.streamProxy.GetSegment(homeID, videoID, segmentName)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, segmentContentType(segmentName), data)
}

func segmentContentType(name string) string {
	if strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".m4s") {
		return "video/mp4"
	}
	return "video/mp2t"
}

func (s *CloudServer) handleCacheStats(c *gin.Context) {
	stats := s.cache.Stats()
	c.JSON(http.StatusOK, stats)
}

func (s *CloudServer) handleStaticFiles(c *gin.Context) {
	path := c.Request.URL.Path
	filePath := filepath.Join(s.distDir, path)

	if _, err := os.Stat(filePath); err == nil {
		c.File(filePath)
		return
	}

	indexPath := filepath.Join(s.distDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		c.File(indexPath)
		return
	}

	c.Status(http.StatusNotFound)
}

type videoJSON struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Duration     int64  `json:"duration"`
	Filesize     int64  `json:"filesize"`
	Resolution   string `json:"resolution"`
	CreatedAt    string `json:"created_at"`
	HomeServerID string `json:"home_server_id"`
	ThumbnailURL string `json:"thumbnail_url"`
	PlayCount    int    `json:"play_count"`
	CommentCount int    `json:"comment_count"`
	Visibility   string `json:"visibility"`
}

func videoToJSON(v *model.Video) videoJSON {
	return videoJSON{
		ID:           v.ID,
		Filename:     v.Filename,
		Title:        v.Title,
		Description:  v.Description,
		Duration:     v.Duration,
		Filesize:     v.Filesize,
		Resolution:   v.Resolution,
		CreatedAt:    v.CreatedAt,
		HomeServerID: v.HomeServerID,
		ThumbnailURL: "/api/thumbnail/" + v.HomeServerID + "/" + v.ID,
		Visibility:   v.Visibility,
	}
}

func videosToJSON(videos []*model.Video) []videoJSON {
	result := make([]videoJSON, len(videos))
	for i, v := range videos {
		result[i] = videoToJSON(v)
	}
	return result
}

func (s *CloudServer) handlePlayVideo(c *gin.Context) {
	videoID := c.Param("videoID")
	if !s.checkVideoAccess(c, videoID) {
		return
	}
	if err := s.videoStore.IncrementPlayCount(videoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *CloudServer) handleGetComments(c *gin.Context) {
	videoID := c.Param("videoID")
	comments, err := s.videoStore.GetComments(videoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, comments)
}

func (s *CloudServer) handleUpdateVideoInfo(c *gin.Context) {
	videoID := c.Param("videoID")

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	video, ok := s.homeRegistry.FindVideoByID(videoID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	updated, err := s.streamProxy.UpdateVideoInfo(video.HomeServerID, videoID, req.Title, req.Description, req.Visibility)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	s.homeRegistry.UpdateVideoInfo(video.HomeServerID, videoID, req.Title, req.Description, req.Visibility)
	c.JSON(http.StatusOK, videoToJSON(updated))
}

func (s *CloudServer) handleAddComment(c *gin.Context) {
	videoID := c.Param("videoID")
	userID := c.GetString("user_id")

	var req struct {
		Content  string `json:"content"`
		ParentID *int   `json:"parent_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	comment, err := s.videoStore.AddComment(videoID, userID, req.Content, req.ParentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.videoStore.IncrementCommentCount(videoID)
	comment.Username = c.GetString("username")
	c.JSON(http.StatusCreated, comment)
}

func (s *CloudServer) handleUploadVideo(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedUploadExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type: " + ext})
		return
	}

	homeID := c.PostForm("home_server_id")
	if homeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "home_server_id is required"})
		return
	}

	sha256Hash := c.PostForm("sha256")
	if sha256Hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sha256 is required"})
		return
	}

	title := c.PostForm("title")
	description := c.PostForm("description")

	videoID, err := s.streamProxy.UploadVideo(homeID, header.Filename, title, description, header.Size, sha256Hash, file)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"video_id": videoID})
}

func (s *CloudServer) handleListHomes(c *gin.Context) {
	homes := s.homeRegistry.ListHomes()
	c.JSON(http.StatusOK, homes)
}

func (s *CloudServer) handleHomeStatus(c *gin.Context) {
	homes := s.homeRegistry.ListHomes()
	c.JSON(http.StatusOK, gin.H{
		"online": len(homes) > 0,
		"homes":  homes,
	})
}

func (s *CloudServer) handleScanHome(c *gin.Context) {
	homeID := c.Param("homeID")
	if err := s.streamProxy.ScanAndConvert(homeID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *CloudServer) handleListUsers(c *gin.Context) {
	users, err := s.videoStore.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (s *CloudServer) handleDeleteUser(c *gin.Context) {
	userID := c.Param("userID")
	if userID == c.GetString("user_id") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}
	if err := s.videoStore.DeleteUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *CloudServer) handleUpdateUserRole(c *gin.Context) {
	userID := c.Param("userID")
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role is required"})
		return
	}
	if err := s.videoStore.UpdateUserRole(userID, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *CloudServer) handleUpdateUserPassword(c *gin.Context) {
	userID := c.Param("userID")
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}
	if err := s.videoStore.UpdateUserPassword(userID, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
