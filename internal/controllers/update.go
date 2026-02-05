package controllers

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/updater"
	"Q115-STRM/internal/v115open"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
)

type version struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Note    string `json:"note"`
	Url     string `json:"url"`
	Current bool   `json:"current"`
	Latest  bool   `json:"latest"`
}

type udpateStatus string

const (
	updateStatusDownloading udpateStatus = "downloading" // 正在下载
	updateStatusInstall     udpateStatus = "install"     // 安装中
)

type updateInfo struct {
	Version     string          `json:"version"`     // 要更新的版本
	DownloadURL string          `json:"downloadURL"` // 下载链接
	Progress    int             `json:"progress"`    // 下载进度
	TotalSize   int64           `json:"total_size"`  // 总大小
	Downloaded  int64           `json:"downloaded"`  // 已下载大小
	Checksum    string          `json:"checksum"`    // 校验和
	Status      string          `json:"status"`      // 状态
	ctx         context.Context `json:"-"`           // 上下文
}

var currentUpdateInfo *updateInfo

// GetLastRelease 获取最新版本列表
// @Summary 获取最新版本
// @Description 获取GitHub上最新的5个稳定版本
// @Tags 更新管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /update/last [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetLastRelease(c *gin.Context) {
	force := c.Query("force")
	passCache := false
	if force == "1" {
		passCache = true
	}
	releases := listReleases(passCache)
	if releases == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取最新版本失败", Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取最新版本成功", Data: releases})
}

// UpdateToVersion 更新到指定版本
// @Summary 更新到指定版本
// @Description 下载并安装指定版本的更新包
// @Tags 更新管理
// @Accept json
// @Produce json
// @Param version body string true "版本号"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /update/to-version [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateToVersion(c *gin.Context) {
	if currentUpdateInfo != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "正在更新中", Data: nil})
		return
	}
	type UpdateVersionRequest struct {
		Version string `json:"version"`
	}
	var req UpdateVersionRequest
	if perr := c.ShouldBindJSON(&req); perr != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	version := req.Version
	if version == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "版本号不能为空", Data: nil})
		return
	}
	// 查询版本信息，然后下载压缩包
	updater := updater.NewGitHubUpdater("qicfan", "qmediasync", helpers.Version)
	downloadURL, _, _, err := updater.GetReleaseDownloadURL(version)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "版本不存在", Data: nil})
		return
	}
	// 测试downloadUrl是否可以连通
	// 先测试链接是否可用
	proxyUrl := helpers.TestGithub(downloadURL, models.SettingsGlobal.HttpProxy)
	if proxyUrl == "failed" && models.SettingsGlobal.HttpProxy == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "无法连通github，也没有设置代理，无法升级", Data: nil})
		return
	}
	if proxyUrl != "failed" {
		downloadURL = proxyUrl
	}
	currentUpdateInfo = &updateInfo{
		Version:     version,
		DownloadURL: downloadURL,
		Progress:    0,
		TotalSize:   0,
		Downloaded:  0,
		Status:      string(updateStatusDownloading),
		ctx:         context.Background(),
	}
	// 启动一个更新协程，然后返回
	go func() {
		defer func() {
			// currentUpdateInfo.ctx.Done()
			currentUpdateInfo = nil
		}()
		updateFilePath := filepath.Join(helpers.ConfigDir, "tmp")
		if helpers.PathExists(updateFilePath) {
			os.MkdirAll(updateFilePath, 0777)
		}
		// 拿到url中的文件名
		filename := filepath.Base(currentUpdateInfo.DownloadURL)
		updateFilename := filepath.Join(updateFilePath, filename)
		if helpers.PathExists(updateFilename) {
			os.Remove(updateFilename)
		}
		httpProxy := ""
		if proxyUrl == "failed" {
			httpProxy = models.SettingsGlobal.HttpProxy
		}
		// 下载文件
		err := helpers.DownloadFileWithProgress(currentUpdateInfo.ctx, httpProxy, currentUpdateInfo.DownloadURL, updateFilename, v115open.DEFAULTUA, func(progress int64, total int64) {
			currentUpdateInfo.Progress = int(float64(progress) / float64(total) * 100)
			currentUpdateInfo.TotalSize = total
			currentUpdateInfo.Downloaded = progress
		})
		if err != nil {
			helpers.AppLogger.Errorf("下载文件失败: %v", err)
			return
		}
		// 检查上下文是否被取消
		select {
		case <-currentUpdateInfo.ctx.Done():
			// 上下文被取消，删除下载的文件
			os.Remove(updateFilename)
			helpers.AppLogger.Infof("更新已取消，删除下载的文件: %s", updateFilename)
			return
		default:
			// 上下文未被取消，继续执行安装
		}
		// 修改为安装中
		currentUpdateInfo.Status = string(updateStatusInstall)
		// 如果是windows, 解压到helpers.ConfigDir/update目录下
		if runtime.GOOS == "windows" {
			updateDestpath := filepath.Join(helpers.ConfigDir, "update")
			if helpers.PathExists(updateDestpath) {
				os.RemoveAll(updateDestpath)
			}
			os.MkdirAll(updateDestpath, 0777)
			// 复制文件到update目录
			goos := runtime.GOOS
			goarch := runtime.GOARCH
			if goarch == "amd64" {
				goarch = "x86_64"
			}
			srcPath := filepath.Join(updateFilePath, "qmediasync_"+goos+"_"+goarch)
			// 解压到updaet目录，然后将文件从qmediasync_GOOS_GOARCH目录下复制到udpate目录
			helpers.ExtractZip(updateFilename, srcPath)
			err = helpers.MoveDir(srcPath, updateDestpath)
			if err != nil {
				helpers.AppLogger.Errorf("移动文件失败: %v", err)
			} else {
				// 文件已复制到更新目录
				helpers.AppLogger.Infof("文件已复制到更新目录: %s", updateDestpath)
			}
			// 删除解压目录
			rerr := os.RemoveAll(srcPath)
			if rerr != nil {
				helpers.AppLogger.Errorf("删除解压目录失败: %v", rerr)

			} else {
				// 解压目录已删除
				helpers.AppLogger.Infof("解压目录已删除: %s", srcPath)
			}
			// 删除压缩包
			err = os.Remove(updateFilename)
			if err != nil {
				helpers.AppLogger.Errorf("删除压缩包失败: %v", err)
			} else {
				// 压缩包已删除
				helpers.AppLogger.Infof("压缩包已删除: %s", updateFilename)
			}
			// 启动更新脚本
			if helpers.IsRelease {
				// 启动更新脚本
				triggerUpdate()
			} else {
				// 模拟更新结束，清除更新信息
				currentUpdateInfo = nil
			}
			return
		}
		if helpers.IsRunningInDocker() {
			folerName := filepath.Base(currentUpdateInfo.DownloadURL)
			folerName = strings.ReplaceAll(folerName, ".tar.gz", "")
			// 重新打包，将压缩包中的文件从qmediasync_GOOS_GOARCH目录下打包到压缩包根目录
			// 解压到updaet目录，然后将文件从qmediasync_GOOS_GOARCH目录下复制到udpate目录
			helpers.ExtractTarGz(updateFilename, updateFilePath)
			// 复制文件到update目录
			srcPath := filepath.Join(helpers.ConfigDir, "tmp", folerName)
			// 给srcPath/scripts下的所有脚本增加执行权限
			scriptsPath := filepath.Join(srcPath, "scripts")
			if helpers.PathExists(scriptsPath) {
				// 遍历scripts目录下的所有文件
				files, rerr := os.ReadDir(scriptsPath)
				if rerr != nil {
					helpers.AppLogger.Errorf("读取目录失败: %v", rerr)
				} else {
					for _, file := range files {
						if file.IsDir() {
							continue
						}
						// 给文件增加执行权限
						err = os.Chmod(filepath.Join(scriptsPath, file.Name()), 0777)
						if err != nil {
							helpers.AppLogger.Errorf("增加执行权限失败: %v", err)
						} else {
							helpers.AppLogger.Infof("文件已增加执行权限: %s", filepath.Join(scriptsPath, file.Name()))
						}
					}
				}
			}
			exeFile := filepath.Join(srcPath, "QMediaSync")
			if helpers.PathExists(exeFile) {
				os.Chmod(exeFile, 0777)
			}

			destFile := filepath.Join(helpers.ConfigDir, "tmp", "qms.update.tar.gz")
			if helpers.PathExists(destFile) {
				os.Remove(destFile)
			}
			// 将srcPath内的文件打包到destFile
			// helpers.CreateTarGz(srcPath, destFile)
			err = exec.Command("tar", "-czvf", destFile, "-C", srcPath, ".").Run()
			if err != nil {
				helpers.AppLogger.Errorf("打包文件失败: %v", err)
			} else {
				// 压缩包已创建
				helpers.AppLogger.Infof("压缩包已创建: %s", destFile)
			}
			// 删除更新目录
			updatePath := filepath.Join(helpers.RootDir, "update")
			if helpers.PathExists(updatePath) {
				os.RemoveAll(updatePath)
				helpers.AppLogger.Infof("已删除老的更新目录: %s", updatePath)
			}
			destGzFile := filepath.Join(helpers.RootDir, "qms.update.tar.gz")
			if helpers.PathExists(destGzFile) {
				os.Remove(destGzFile)
			}
			// 将文件移动到rootDir
			err = helpers.CopyFile(destFile, destGzFile)
			if err != nil {
				helpers.AppLogger.Errorf("移动文件失败: %v", err)
			} else {
				// 文件已移动到更新目录
				helpers.AppLogger.Infof("文件已移动到更新目录: %s", destGzFile)
			}
			// 删除解压目录
			err = os.RemoveAll(srcPath)
			if err != nil {
				helpers.AppLogger.Errorf("删除解压目录失败: %v", err)
			} else {
				// 解压目录已删除
				helpers.AppLogger.Infof("解压目录已删除: %s", srcPath)
			}
			// 删除下载文件
			err = os.Remove(updateFilename)
			if err != nil {
				helpers.AppLogger.Errorf("删除压缩包失败: %v", err)
			} else {
				// 压缩包已删除
				helpers.AppLogger.Infof("压缩包已删除: %s", updateFilename)
			}
			err = os.Remove(destFile)
			if err != nil {
				helpers.AppLogger.Errorf("删除压缩包失败: %v", err)
			} else {
				// 压缩包已删除
				helpers.AppLogger.Infof("压缩包已删除: %s", destFile)
			}
			// 等待更新器重启应用
			currentUpdateInfo = nil
		}
	}()
	// 返回更新信息
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新开始", Data: currentUpdateInfo})
}

// UpdateProgress 获取更新进度
// @Summary 获取更新进度
// @Description 查询当前更新任务的下载和安装进度
// @Tags 更新管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /update/progress [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateProgress(c *gin.Context) {
	if currentUpdateInfo == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未开始更新", Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新进度", Data: currentUpdateInfo})
}

// CancelUpdate 取消更新
// @Summary 取消更新
// @Description 取消正在进行的更新任务
// @Tags 更新管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /update/cancel [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CancelUpdate(c *gin.Context) {
	if currentUpdateInfo == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未开始更新", Data: nil})
		return
	}
	// 取消更新
	currentUpdateInfo.ctx.Done()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新已取消", Data: nil})
}

// listReleases 列出最新版本
func listReleases(passCache bool) []version {
	// 直接读取缓存
	if !passCache {
		cached := db.Cache.Get("latest_releases")
		if cached != nil {
			helpers.AppLogger.Infof("使用缓存的最新版本列表")
			var versionList []version
			err := json.Unmarshal(cached, &versionList)
			if err == nil {
				return versionList
			} else {
				helpers.AppLogger.Infof("解析缓存的最新版本列表失败: %v", err)
			}
		}
	}
	updater := updater.NewGitHubUpdater("qicfan", "qmediasync", helpers.Version)
	updater.IncludePreRelease = false

	releases, err := updater.GetLatestStableReleases(5)
	if err != nil {
		helpers.AppLogger.Errorf("查找Github最新版本失败: %v", err)
		return nil
	}

	if len(releases) == 0 {
		helpers.AppLogger.Infof("未找到最新版本")
		return nil
	}

	helpers.AppLogger.Infof("找到  %s/%s 的 %d 个最新版本", "qicfan", "qmediasync", len(releases))
	// helpers.AppLogger.Infof("==============================================")
	versionList := make([]version, 0)
	for i, release := range releases {
		versionList = append(versionList, version{
			Version: release.Version,
			Date:    release.PublishedAt.Format("2006-01-02 15:04:05"),
			Note:    release.ReleaseNotes,
			Url:     release.PageURL,
			Current: release.Version == helpers.Version,
			Latest:  i == 0,
		})
	}
	// 缓存1小时
	versionListStr, _ := json.Marshal(versionList)
	if versionListStr != nil {
		db.Cache.Set("latest_releases", versionListStr, 3600)
	}
	return versionList
}

func triggerUpdate() {
	exePath, err := os.Executable()
	if err != nil {
		helpers.AppLogger.Errorf("获取程序路径失败: %v", err)
		return
	}
	updateDir := filepath.Join(helpers.ConfigDir, "update")
	if !helpers.PathExists(updateDir) {
		helpers.AppLogger.Errorf("更新目录不存在: %s", updateDir)
		return
	}

	if !helpers.StartNewProcess(exePath, updateDir) {
		helpers.AppLogger.Errorf("启动更新进程失败: %v", err)
		return
	}

	helpers.StopApp()
}
