package router

import (
	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/handler"
	"github.com/agentdisk/agent-disk/internal/middleware"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/oauth2client"
	"github.com/agentdisk/agent-disk/pkg/oss"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// Setup handles the operation.
func Setup(cfg *config.Config) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	// Init dependencies
	db, err := repository.InitDB(cfg)
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
	ossClient, err := oss.NewClient(
		cfg.OSS.Endpoint,
		cfg.OSS.AccessKey,
		cfg.OSS.SecretKey,
		cfg.OSS.Bucket,
		cfg.OSS.Region,
		cfg.OSS.UseSSL,
	)
	if err != nil {
		panic("failed to create OSS client: " + err.Error())
	}

	// OAuth2 client (optional)
	var authH *handler.AuthHandler
	if cfg.OAuth2.Enabled {
		oauthClient := oauth2client.New(oauth2client.Config{
			ClientID:     cfg.OAuth2.ClientID,
			ClientSecret: cfg.OAuth2.ClientSecret,
			AuthURL:      cfg.OAuth2.AuthURL,
			TokenURL:     cfg.OAuth2.TokenURL,
			UserInfoURL:  cfg.OAuth2.UserInfoURL,
			RedirectURL:  cfg.OAuth2.RedirectURL,
			Scopes:       cfg.OAuth2.Scopes,
		})
		authH = handler.NewAuthHandler(oauthClient, cfg.OAuth2.FrontendURL)
	}

	// Repos
	spaceRepo := repository.NewSpaceRepo(db)
	folderRepo := repository.NewFolderRepo(db)
	fileRepo := repository.NewFileRepo(db)
	permRepo := repository.NewPermissionRepo(db)
	versionRepo := repository.NewVersionRepo(db)
	recycleRepo := repository.NewRecycleRepo(db)
	tagRepo := repository.NewTagRepo(db)
	shareRepo := repository.NewShareRepo(db)

	// Services
	spaceSvc := service.NewSpaceService(spaceRepo)
	folderSvc := service.NewFolderService(folderRepo, ossClient)
	fileSvc := service.NewFileService(fileRepo, folderRepo, versionRepo, spaceRepo, ossClient)
	permSvc := service.NewPermissionService(permRepo)
	versionSvc := service.NewVersionService(versionRepo, fileRepo, ossClient)
	recycleSvc := service.NewRecycleService(recycleRepo, fileRepo, folderRepo, spaceRepo, ossClient)
	tagSvc := service.NewTagService(tagRepo)
	shareSvc := service.NewShareService(shareRepo, fileRepo, folderRepo)
	previewSvc := service.NewPreviewService(fileSvc, ossClient)

	// Handlers
	spaceH := handler.NewSpaceHandler(spaceSvc)
	folderH := handler.NewFolderHandler(folderSvc, recycleSvc)
	fileH := handler.NewFileHandler(fileSvc, recycleSvc, cfg.DownloadToken.Secret, cfg.DownloadToken.ExpireSeconds)
	permH := handler.NewPermissionHandler(permSvc)
	versionH := handler.NewVersionHandler(versionSvc)
	recycleH := handler.NewRecycleHandler(recycleSvc)
	tagH := handler.NewTagHandler(tagSvc)
	shareH := handler.NewShareHandler(shareSvc)
	previewH := handler.NewPreviewHandler(previewSvc)

	// OAuth2 auth routes (public)
	if authH != nil {
		r.GET("/auth/login", authH.Login)
		r.GET("/auth/callback", authH.Callback)
		r.POST("/auth/logout", authH.Logout)
	}

	// API v1 group with hybrid auth
	v1 := r.Group("/v1/disk")
	v1.Use(middleware.HybridAuth(cfg.JWT.Secret, authH, cfg.DownloadToken.Secret))
	// Space
	v1.GET("/space", spaceH.GetSpace)

	// Folders
	v1.POST("/folders", folderH.CreateFolder)
	v1.GET("/folders", folderH.ListFolders)
	v1.GET("/folders/:id", folderH.GetFolder)
	v1.GET("/folders/:id/ancestors", folderH.GetAncestors)
	v1.PUT("/folders/:id", folderH.RenameFolder)
	v1.DELETE("/folders/:id", folderH.DeleteFolder)

	// Files
	v1.POST("/files/upload", fileH.UploadFile)
	v1.GET("/files/:id", fileH.GetFile)
	v1.PUT("/files/:id", fileH.UpdateFile)
	v1.DELETE("/files/:id", fileH.DeleteFile)
	v1.GET("/files", fileH.ListFiles)
	v1.POST("/files/:id/download-token", fileH.CreateDownloadToken)

	// Permissions
	v1.POST("/permissions", permH.GrantPermission)
	v1.GET("/permissions/check", permH.CheckPermission)
	v1.DELETE("/permissions", permH.RevokePermission)
	v1.GET("/permissions", permH.ListPermissions)

	// Versions
	v1.GET("/versions", versionH.ListVersions)
	v1.POST("/versions/rollback", versionH.RollbackVersion)

	// Recycle bin
	v1.GET("/recycle", recycleH.ListRecycle)
	v1.POST("/recycle/restore", recycleH.RestoreItem)
	v1.DELETE("/recycle", recycleH.DeletePermanent)

	// Tags
	v1.POST("/tags/bind", tagH.BindTag)
	v1.POST("/tags/unbind", tagH.UnbindTag)
	v1.GET("/tags/search", tagH.SearchByTags)

	// Shares
	v1.POST("/shares", shareH.CreateShare)
	v1.GET("/shares", shareH.ListShares)
	v1.DELETE("/shares", shareH.RevokeShare)

	// Preview
	v1.GET("/preview/:id", previewH.PreviewFile)

	// Public routes (no auth required)
	r.GET("/v1/disk/share/:code", shareH.GetShare)
	r.POST("/v1/disk/share/access", shareH.AccessShare)
	r.GET("/v1/disk/files/download", fileH.DownloadByToken)

	return r
}
