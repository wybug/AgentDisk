package router

import (
	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/handler"
	"github.com/agentdisk/agent-disk/internal/middleware"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/oss"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

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
	recycleSvc := service.NewRecycleService(recycleRepo, fileRepo, folderRepo, ossClient)
	tagSvc := service.NewTagService(tagRepo)
	shareSvc := service.NewShareService(shareRepo)
	previewSvc := service.NewPreviewService(fileSvc, ossClient)

	// Handlers
	spaceH := handler.NewSpaceHandler(spaceSvc)
	folderH := handler.NewFolderHandler(folderSvc)
	fileH := handler.NewFileHandler(fileSvc)
	permH := handler.NewPermissionHandler(permSvc)
	versionH := handler.NewVersionHandler(versionSvc)
	recycleH := handler.NewRecycleHandler(recycleSvc)
	tagH := handler.NewTagHandler(tagSvc)
	shareH := handler.NewShareHandler(shareSvc)
	previewH := handler.NewPreviewHandler(previewSvc)

	// API v1 group with JWT auth
	v1 := r.Group("/v1/disk")
	v1.Use(middleware.JWTAuth(cfg.JWT.Secret))
	{
		// Space
		v1.GET("/space", spaceH.GetSpace)

		// Folders
		v1.POST("/folders", folderH.CreateFolder)
		v1.GET("/folders", folderH.ListFolders)
		v1.DELETE("/folders/:id", folderH.DeleteFolder)

		// Files
		v1.POST("/files/upload", fileH.UploadFile)
		v1.GET("/files/:id", fileH.GetFile)
		v1.PUT("/files/:id", fileH.UpdateFile)
		v1.DELETE("/files/:id", fileH.DeleteFile)
		v1.GET("/files", fileH.ListFiles)

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
	}

	// Public share access (no auth required)
	r.GET("/v1/disk/share/:code", shareH.GetShare)
	r.POST("/v1/disk/share/access", shareH.AccessShare)

	return r
}
