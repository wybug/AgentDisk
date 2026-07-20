package router

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/feature"
	"github.com/agentdisk/agent-disk/internal/handler"
	"github.com/agentdisk/agent-disk/internal/middleware"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/internal/store"
	"github.com/agentdisk/agent-disk/pkg/oss"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/agentdisk/agent-disk/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Setup builds the gin engine, wires all services, and starts the feature
// flag hot-reload watcher. cfgPath is the on-disk config.yaml path; the
// feature registry watches it for external edits and persists admin-initiated
// toggles back to it. Callers should defer featureReg.Close().
func Setup(cfg *config.Config, cfgPath string) (*gin.Engine, *feature.Registry, error) {
	featureReg, err := feature.NewRegistry(cfg, cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("feature registry: %w", err)
	}
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
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
	if migrateErr := repository.AutoMigrate(db); migrateErr != nil {
		panic("failed to auto-migrate database: " + migrateErr.Error())
	}

	// Init storage backend
	storageDriver := cfg.OSS.Driver
	if storageDriver == "" {
		storageDriver = "minio"
	}

	var fileStorage storage.Storage

	switch storageDriver {
	case "local":
		rootDir := cfg.OSS.Path
		if rootDir == "" {
			homeDir, _ := os.UserHomeDir()
			rootDir = filepath.Join(homeDir, ".agentdisk", "disk")
		}
		localStorage := storage.NewLocalStorage(rootDir, cfg.JWT.Secret)
		if err := localStorage.EnsureBucket(context.Background()); err != nil {
			panic("failed to init local storage: " + err.Error())
		}
		fileStorage = localStorage
		localH := storage.NewLocalStorageHandler(localStorage)
		r.GET("/v1/disk/local-storage/*key", localH.ServeFile)
	default:
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
		fileStorage = storage.NewMinioStorage(ossClient)
		if err := fileStorage.EnsureBucket(context.Background()); err != nil {
			panic("failed to ensure bucket: " + err.Error())
		}
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
	adminRepo := repository.NewAdminRepo(db)
	apiKeyRepo := repository.NewAPIKeyRepo(db)
	publicDirRepo := repository.NewPublicDirectoryRepo(db)
	grantRepo := repository.NewPublicDirectoryGrantRepo(db)
	oauth2ConfigRepo := repository.NewOAuth2ConfigRepo(db)
	passkeyRepo := repository.NewAdminPasskeyRepo(db)
	okfBundleRepo := repository.NewOkfBundleRepo(db)
	okfNodeRepo := repository.NewOkfNodeRepo(db)
	okfEdgeRepo := repository.NewOkfEdgeRepo(db)

	// Services
	spaceSvc := service.NewSpaceService(spaceRepo)
	folderSvc := service.NewFolderService(folderRepo)
	fileSvc := service.NewFileService(fileRepo, folderRepo, versionRepo, spaceRepo, fileStorage)
	permSvc := service.NewPermissionService(permRepo)
	versionSvc := service.NewVersionService(versionRepo, fileRepo, fileStorage)
	recycleSvc := service.NewRecycleService(recycleRepo, fileRepo, folderRepo, spaceRepo, fileStorage)
	tagSvc := service.NewTagService(tagRepo)
	shareSvc := service.NewShareService(shareRepo, fileRepo, folderRepo)
	previewSvc := service.NewPreviewService(fileSvc, fileStorage)
	adminSvc := service.NewAdminService(adminRepo)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo)
	publicDirSvc := service.NewPublicDirectoryService(publicDirRepo, folderRepo, grantRepo, fileRepo, fileStorage, cfg.DownloadToken.Secret, cfg.DownloadToken.ExpireSeconds)
	shareSvc.SetGrantChecker(publicDirSvc)
	oauth2ConfigSvc := service.NewOAuth2ConfigService(oauth2ConfigRepo)
	// OKF service shares the public directory service for storage + folder
	// lookups, so the original pdWrite behavior is unchanged when OKF is
	// disabled at the route layer. db is the same GORM handle the repos use, so
	// the OKF service can wrap multi-step materialization in a transaction.
	okfSvc := service.NewOkfService(okfBundleRepo, okfNodeRepo, okfEdgeRepo, publicDirSvc, cfg.Database.Driver, db)
	// okfShareReaderSvc is the read-only OKF accessor for share-code
	// endpoints. It reuses the same repos as okfSvc but skips the HybridAuth
	// visibility check — possession of a valid share code IS the grant.
	okfShareReaderSvc := service.NewOkfShareReader(okfBundleRepo, okfNodeRepo, okfEdgeRepo, publicDirSvc, cfg.Database.Driver)
	// Extend ShareService to accept bundle as a shareable resType. The grant
	// checker is the same publicDirSvc the file-share path uses; we wire the
	// bundle repo so the bundle case in CreateShare can look up PublicDirectoryID.
	shareSvc.SetBundleRepo(okfBundleRepo)
	okfSvc.SetAutoIndexUpdate(cfg.Okf.AutoIndexUpdate)
	if cfg.Okf.LockTTLSeconds > 0 || cfg.Okf.RedisAddr != "" {
		// Wire the Redis-backed bundle lock when an address is configured. A
		// missing address falls back to the no-op lock (last-writer-wins) so
		// single-writer deployments are not forced to run Redis just for OKF.
		if cfg.Okf.RedisAddr != "" {
			redisClient := redis.NewClient(&redis.Options{Addr: cfg.Okf.RedisAddr})
			// Ping once so a misconfigured address surfaces at startup rather
			// than as a 500 on the first write. On failure we log and fall
			// back to the no-op lock so the process still serves reads.
			if pErr := redisClient.Ping(context.Background()).Err(); pErr != nil {
				log.Printf("warning: okf redis ping failed (%v); writer lock falls back to no-op", pErr)
			} else {
				okfSvc.SetBundleLock(service.NewRedisBundleLock(redisClient, cfg.Okf.LockTTLSeconds))
				// Wire the BFS adjacency cache on the same Redis client. A warm
				// cache lets Reachable/Subgraph/Neighbors skip the per-hop edge
				// lookup; WriteMarkdown invalidates the touched entry on commit.
				// Without this call the cache defaulted to NoOp and every BFS hop
				// hit the DB regardless of cfg.Okf.RedisAddr.
				okfSvc.SetGraphCache(service.NewRedisGraphCache(redisClient, cfg.Okf.AdjTTLSeconds))
			}
		}
	}

	// OAuth2 auth handler (reads DB config per-request for hot-reload)
	authH := handler.NewAuthHandler(oauth2ConfigSvc, cfg.Server.FrontendURL)

	// Handlers
	spaceH := handler.NewSpaceHandler(spaceSvc)
	folderH := handler.NewFolderHandler(folderSvc, recycleSvc)
	fileH := handler.NewFileHandler(fileSvc, permSvc, recycleSvc, cfg.DownloadToken.Secret, cfg.DownloadToken.ExpireSeconds)
	permH := handler.NewPermissionHandler(permSvc)
	versionH := handler.NewVersionHandler(versionSvc)
	recycleH := handler.NewRecycleHandler(recycleSvc)
	tagH := handler.NewTagHandler(tagSvc)
	shareH := handler.NewShareHandler(shareSvc, cfg.DownloadToken.Secret, cfg.DownloadToken.ExpireSeconds)
	previewH := handler.NewPreviewHandler(previewSvc)
	adminH := handler.NewAdminHandler(adminSvc, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	apiKeyH := handler.NewAPIKeyHandler(apiKeySvc)
	publicDirH := handler.NewPublicDirectoryHandler(publicDirSvc)
	publicDirContentH := handler.NewPublicDirectoryContentHandler(okfSvc)
	okfH := handler.NewOkfHandler(okfSvc)
	okfShareH := handler.NewOkfShareHandler(okfShareReaderSvc, shareSvc)
	okfScanH := handler.NewOkfScanHandler(okfSvc)
	// Retry-After on a 409 lock-held should hint the configured lock lifetime so
	// clients back off for the right duration rather than a hardcoded guess.
	okfH.SetLockRetryAfter(cfg.Okf.LockTTLSeconds)
	okfScanH.SetLockRetryAfter(cfg.Okf.LockTTLSeconds)
	oauth2ConfigH := handler.NewOAuth2ConfigHandler(oauth2ConfigSvc)
	featureH := handler.NewFeatureHandler(featureReg)

	// WebAuthn MFA (optional, enabled via config)
	var mfaH *handler.AdminMFAHandler
	if cfg.WebAuthn.Enabled {
		sessionStore := store.NewWebAuthnSessionStore()
		mfaSvc, err := service.NewAdminMFAService(cfg.WebAuthn, passkeyRepo, adminRepo, sessionStore, cfg.JWT.Secret, cfg.JWT.ExpireHours)
		if err != nil {
			panic("failed to create MFA service: " + err.Error())
		}
		mfaH = handler.NewAdminMFAHandler(mfaSvc, cfg.JWT.Secret)
		adminH.SetMFAService(mfaSvc)
	}

	// OAuth2 status endpoint (public, always available)
	r.GET("/auth/status", authH.Status)

	// OAuth2 auth routes (public, always registered)
	r.GET("/auth/login", authH.Login)
	r.GET("/auth/callback", authH.Callback)
	r.POST("/auth/logout", authH.Logout)

	// Admin login (public, no auth required)
	r.POST("/v1/disk/admin/login", adminH.Login)
	r.POST("/v1/disk/admin/bootstrap", adminH.Bootstrap)
	r.GET("/v1/disk/admin/init-status", adminH.InitStatus)

	// MFA login routes (public, session token based)
	if mfaH != nil {
		r.POST("/v1/disk/admin/mfa/login/begin", mfaH.BeginMFALogin)
		r.POST("/v1/disk/admin/mfa/login/finish", mfaH.FinishMFALogin)
	}

	// Admin management routes (AdminAuth + AdminOnly)
	adminAPI := r.Group("/v1/disk/admin")
	adminAPI.Use(middleware.AdminAuth(cfg.JWT.Secret))
	adminAPI.Use(middleware.AdminOnly())
	{
		adminAPI.GET("/dashboard", adminH.Dashboard)
		adminAPI.GET("/users", adminH.ListUsers)
		adminAPI.POST("/users", adminH.CreateUser)
		adminAPI.PUT("/users/:username/password", adminH.ChangePassword)
		adminAPI.DELETE("/users/:username", adminH.DeleteUser)

		// MFA/WebAuthn routes (only when enabled)
		if mfaH != nil {
			mfa := adminAPI.Group("/mfa")
			mfa.POST("/registration/begin", mfaH.BeginRegistration)
			mfa.POST("/registration/finish", mfaH.FinishRegistration)
			mfa.GET("/credentials", mfaH.ListPasskeys)
			mfa.DELETE("/credentials/:id", mfaH.DeletePasskey)
			mfa.PUT("/credentials/:id", mfaH.RenamePasskey)
			mfa.GET("/status", mfaH.GetMFAStatus)
			mfa.PUT("/enabled", mfaH.SetMFAEnabled)
		}

		keys := adminAPI.Group("/api-keys")
		keys.POST("", apiKeyH.Create)
		keys.GET("", apiKeyH.List)
		keys.PUT("/:id", apiKeyH.Update)
		keys.DELETE("/:id", apiKeyH.Revoke)

		pd := adminAPI.Group("/public-directories")
		pd.POST("", publicDirH.Create)
		pd.GET("", publicDirH.List)
		pd.PUT("/:id", publicDirH.Update)
		pd.DELETE("/:id", publicDirH.Delete)

		adminAPI.GET("/oauth2", oauth2ConfigH.Get)
		adminAPI.PUT("/oauth2", oauth2ConfigH.Update)
		adminAPI.POST("/oauth2/test", oauth2ConfigH.Test)

		// P5c.2 runtime feature flags. GET lists all flags + their on/off
		// state; PATCH flips one and persists to config.yaml so the toggle
		// survives restart. fsnotify reloads external edits within 200ms.
		adminAPI.GET("/features", featureH.List)
		adminAPI.PATCH("/features", featureH.Update)

		// P3b admin-only OKF graph rebuild. Mounts under adminAPI so the
		// AdminAuth + AdminOnly middleware gate it; the OKF reader group
		// cannot reach it. RefreshBundle does the heavy lift — it walks the
		// folder tree and re-materializes every node + edge in one tx.
		if cfg.Okf.Enabled {
			adminAPI.POST("/okf/bundles/:id/rebuild-graph", okfH.RebuildGraph)
		}
	}

	// API v1 group with hybrid auth
	v1 := r.Group("/v1/disk")
	v1.Use(middleware.HybridAuth(cfg.JWT.Secret, authH, cfg.DownloadToken.Secret, apiKeySvc))

	// Private endpoints — JWT only (block API Key)
	private := v1.Group("")
	private.Use(middleware.RequireNonAPIKey())
	// Space
	private.GET("/space", spaceH.GetSpace)

	// Folders
	private.POST("/folders", folderH.CreateFolder)
	private.GET("/folders", folderH.ListFolders)
	private.GET("/folders/:id", folderH.GetFolder)
	private.GET("/folders/:id/ancestors", folderH.GetAncestors)
	private.PUT("/folders/:id", folderH.RenameFolder)
	private.DELETE("/folders/:id", folderH.DeleteFolder)

	// Files
	private.POST("/files/upload", fileH.UploadFile)
	private.GET("/files/:id", fileH.GetFile)
	private.PUT("/files/:id", fileH.UpdateFile)
	private.DELETE("/files/:id", fileH.DeleteFile)
	private.GET("/files", fileH.ListFiles)
	private.POST("/files/:id/download-token", fileH.CreateDownloadToken)

	// Permissions
	private.POST("/permissions", permH.GrantPermission)
	private.GET("/permissions/check", permH.CheckPermission)
	private.DELETE("/permissions", permH.RevokePermission)
	private.GET("/permissions", permH.ListPermissions)

	// Versions
	private.GET("/versions", versionH.ListVersions)
	private.POST("/versions/rollback", versionH.RollbackVersion)

	// Recycle bin
	private.GET("/recycle", recycleH.ListRecycle)
	private.POST("/recycle/restore", recycleH.RestoreItem)
	private.DELETE("/recycle", recycleH.DeletePermanent)

	// Tags
	private.POST("/tags/bind", tagH.BindTag)
	private.POST("/tags/unbind", tagH.UnbindTag)
	private.GET("/tags/search", tagH.SearchByTags)

	// Shares
	private.POST("/shares", shareH.CreateShare)
	private.GET("/shares", shareH.ListShares)
	private.DELETE("/shares", shareH.RevokeShare)

	// Preview
	private.GET("/preview/:id", previewH.PreviewFile)
	private.GET("/preview/:id/html", previewH.PreviewHTMLFile)

	// Public directories — readable by JWT (需授权) + API Key
	pdRead := v1.Group("/public-directories")
	pdRead.GET("", publicDirH.ListVisible)
	pdRead.GET("/:id", publicDirH.Get)
	pdRead.GET("/:id/folders", publicDirH.ListSubFolders)
	pdRead.GET("/:id/files", publicDirH.ListFiles)
	pdRead.POST("/:id/download-token", publicDirH.CreateDownloadToken)

	// Public directories — writable by API Key only
	pdWrite := v1.Group("/public-directories")
	pdWrite.Use(middleware.RequireAPIKey())
	pdWrite.POST("/:id/files/upload", publicDirH.UploadFile)
	pdWrite.POST("/:id/folders", publicDirH.CreateSubFolder)
	pdWrite.DELETE("/:id/files/:fileId", publicDirH.DeleteFile)

	// OKF v0.1 reader + bundle management (HybridAuth: JWT or API Key). The
	// whole group is gated by config.Okf.Enabled so operators can disable OKF
	// without rebuilding. The content-writer route lives here too because it
	// materializes nodes through the OKF service, so it must stay disabled when
	// OKF is off — otherwise an API-keyed writer could trigger materialization
	// even after operators flipped the OKF switch off.
	//
	// On top of the boot-time gate, three runtime feature flags control
	// sub-groups so admins can shed load without a restart:
	//   - okfReader: reader + maintenance routes
	//   - okfWriter: writer routes (write_content, register, refresh, etc.)
	//   - okfGraphBFS: the BFS-heavy routes (reachable, shortest, subgraph,
	//     neighbors, stats). Disabling these leaves the lighter reader working.
	if cfg.Okf.Enabled {
		// OKF content writer — gated by okfWriter. Lives on pdWrite (which is
		// already API-Key gated) because materialization must use API-key auth.
		pdWrite.POST("/:id/files/content",
			middleware.RequireFeature(featureReg, feature.FlagOkfWriter),
			publicDirContentH.WriteContent)

		okfGroup := v1.Group("/okf")
		okfGroup.Use(middleware.HybridAuth(cfg.JWT.Secret, authH, cfg.DownloadToken.Secret, apiKeySvc))

		// Reader + maintenance routes — gated by okfReader.
		okfReader := okfGroup.Group("",
			middleware.RequireFeature(featureReg, feature.FlagOkfReader))
		okfReader.GET("/bundles", okfH.ListBundles)
		okfReader.GET("/bundles/:id", okfH.GetBundle)
		okfReader.GET("/bundles/:id/nodes", okfH.ListNodes)
		okfReader.GET("/types", okfH.AggregateTypes)
		okfReader.POST("/bundles/:id/scan", okfScanH.ScanBundle)
		okfReader.GET("/bundles/:id/broken-links", okfScanH.ListBrokenLinks)
		okfReader.POST("/search", okfH.Search)

		// Writer routes — gated by okfWriter (register/refresh/unregister plus
		// the index regen, which is itself a write).
		okfWriter := okfGroup.Group("",
			middleware.RequireFeature(featureReg, feature.FlagOkfWriter))
		okfWriter.POST("/bundles/register", okfH.RegisterBundle)
		okfWriter.POST("/bundles/:id/refresh", okfH.RefreshBundle)
		okfWriter.DELETE("/bundles/:id", okfH.UnregisterBundle)
		okfWriter.POST("/bundles/:id/regenerate-index", okfScanH.RegenerateIndex)

		// Graph BFS routes — gated by okfGraphBFS. These are the most CPU-
		// intensive OKF endpoints; an operator shedding load can flip just
		// this flag and keep the reader working.
		okfBFS := okfGroup.Group("",
			middleware.RequireFeature(featureReg, feature.FlagOkfGraphBFS))
		okfBFS.GET("/nodes/:id/neighbors", okfH.Neighbors)
		okfBFS.POST("/nodes/:id/reachable", okfH.Reachable)
		okfBFS.POST("/paths/shortest", okfH.ShortestPath)
		okfBFS.POST("/subgraph", okfH.Subgraph)
		okfBFS.GET("/bundles/:id/stats", okfH.Stats)
	}

	// Public directory grants — API Key only
	pdGrants := v1.Group("/public-directories")
	pdGrants.Use(middleware.RequireAPIKey())
	pdGrants.POST("/:id/grants", publicDirH.GrantAccess)
	pdGrants.DELETE("/:id/grants/:userId", publicDirH.RevokeAccess)
	pdGrants.GET("/:id/grants", publicDirH.ListGrantedUsers)

	// Public routes (no auth required)
	r.GET("/v1/disk/share/:code", shareH.GetShare)
	r.POST("/v1/disk/share/access", shareH.AccessShare)
	r.POST("/v1/disk/share/download", shareH.ShareDownload)
	r.GET("/v1/disk/files/download", fileH.DownloadByToken)

	// OKF bundle share routes (no auth required). Possession of the share
	// code is the credential; the handler re-validates the share + extract
	// code on each call. Mounted only when OKF is enabled so operators
	// disabling OKF also disable bundle sharing.
	if cfg.Okf.Enabled {
		r.GET("/v1/disk/share/:code/bundle", okfShareH.GetShareBundle)
		r.GET("/v1/disk/share/:code/nodes", okfShareH.ListShareNodes)
		r.GET("/v1/disk/share/:code/subgraph", okfShareH.GetShareSubgraph)
		r.GET("/v1/disk/share/:code/nodes/:nodeId/neighbors", okfShareH.GetShareNodeNeighbors)
		r.GET("/v1/disk/share/:code/nodes/:nodeId", okfShareH.GetShareNode)
	}

	return r, featureReg, nil
}
