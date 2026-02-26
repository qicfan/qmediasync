package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/disk"
)

type DirResp struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetPathList 获取目录列表
// @Summary 获取目录列表
// @Description 按同步源类型获取本地、OpenList或115的目录列表
// @Tags 路径管理
// @Accept json
// @Produce json
// @Param parent_id query string false "父目录ID，仅115使用"
// @Param parent_path query string false "父目录路径，本地或OpenList使用"
// @Param source_type query integer true "同步源类型，0-本地 1-115 2-OpenList"
// @Param account_id query integer false "账号ID，115或OpenList必填"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /path/list [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetPathList(c *gin.Context) {
	type dirListReq struct {
		ParentId   string            `json:"parent_id" form:"parent_id"`
		ParentPath string            `json:"parent_path" form:"parent_path"`
		SourceType models.SourceType `json:"source_type" form:"source_type"`
		AccountId  uint              `json:"account_id" form:"account_id"`
		Page       int               `json:"page" form:"page"`
		PageSize   int               `json:"page_size" form:"page_size"`
	}
	var req dirListReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	var pathes []DirResp
	var err error
	switch req.SourceType {
	case models.SourceTypeLocal:
		pathes, err = GetLocalPath(req.ParentId)
	case models.SourceTypeOpenList:
		pathes, err = GetOpenListPath(req.ParentId, req.AccountId)
	case models.SourceType115:
		pathes, err = Get115PathList(req.ParentId, req.AccountId)
	case models.SourceTypeBaiduPan:
		pathes, err = GetBaiduPanPathList(req.ParentId, req.AccountId)
	default:
		// 报错
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未知的同步源类型", Data: nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取目录列表失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取目录列表成功", Data: pathes})
}

// 获取本地目录列表
// parentPath string 父目录路径
// windows，如果parentPath为空，则获取盘符列表
// 非windows，如果parentPath为空，则获取根目录/的子目录列表
func GetLocalPath(parentPath string) ([]DirResp, error) {
	pathes := make([]DirResp, 0)
	// windows
	if parentPath == "" {
		if runtime.GOOS == "windows" {
			helpers.AppLogger.Infof("parentPath: %s", parentPath)
			if parentPath == "" {
				// 获取盘符列表
				partitions, err := disk.Partitions(false)
				helpers.AppLogger.Infof("partitions: %+v", partitions)
				if err != nil {
					helpers.AppLogger.Errorf("获取盘符失败：%v", err)
					return nil, err
				}
				for _, partition := range partitions {
					// helpers.AppLogger.Debugf("盘符: %s", partition.Mountpoint)
					pathes = append(pathes, DirResp{
						Id:   partition.Mountpoint + "\\",
						Name: partition.Mountpoint,
						Path: partition.Mountpoint + "\\",
					})
				}
				return pathes, nil
			}
		} else {
			if helpers.IsFnOS {
				// 如果是飞牛环境下，使用环境变量来获取有权限的目录
				if helpers.AccessiblePathes == "" {
					helpers.AccessiblePathes = os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS")
				}
				// if helpers.SharePathes == "" {
				helpers.SharePathes = os.Getenv("TRIM_DATA_SHARE_PATHS")
				// }
				helpers.AppLogger.Infof("AccessiblePathes: %s", helpers.AccessiblePathes)
				helpers.AppLogger.Infof("SharePathes: %s", helpers.SharePathes)
				if helpers.AccessiblePathes != "" || helpers.SharePathes != "" {
					accessiblePaths := helpers.AccessiblePathes
					sharePaths := helpers.SharePathes
					if sharePaths != "" {
						accessiblePaths += ":" + sharePaths
					}
					helpers.AppLogger.Infof("合并后有权限访问的目录为: %s", accessiblePaths)
					// 用冒号分割
					paths := strings.Split(accessiblePaths, ":")
					for _, path := range paths {
						// 去掉首尾空格
						path = strings.TrimSpace(path)
						// 加入列表
						pathes = append(pathes, DirResp{
							Id:   path,
							Name: path,
							Path: path,
						})
					}
				}
				return pathes, nil
			} else {
				// 获取根目录/的子目录列表
				parentPath = "/"
			}
		}
	}
	// 获取子目录列表
	entries, err := os.ReadDir(parentPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			// 跳过隐藏目录
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			fullPath := filepath.ToSlash(filepath.Join(parentPath, entry.Name()))
			pathes = append(pathes, DirResp{
				Id:   fullPath,
				Name: entry.Name(),
				Path: fullPath,
			})
		}
	}

	return pathes, nil
}

func GetOpenListPath(parentPath string, accountId uint) ([]DirResp, error) {
	account, err := models.GetAccountById(accountId)
	if err != nil {
		return nil, err
	}
	// 去掉parentPath末尾的/
	parentPath = strings.TrimSuffix(parentPath, "/")
	parentPath = strings.TrimSuffix(parentPath, "\\")

	helpers.AppLogger.Infof("开始获取OpenList目录列表, 父目录路径: %s", parentPath)
	client := account.GetOpenListClient()
	resp, err := client.FileList(context.Background(), parentPath, 1, 100)
	if err != nil {
		return nil, err
	}
	// 只返回文件夹列表
	folders := make([]DirResp, 0)
	for _, item := range resp.Content {
		if item.IsDir {
			folders = append(folders, DirResp{
				Id:   parentPath + "/" + item.Name,
				Name: item.Name,
				Path: parentPath + "/" + item.Name,
			})
		}
	}
	return folders, nil
}

func Get115PathList(parentId string, accountId uint) ([]DirResp, error) {
	// 获取115目录列表
	account, err := models.GetAccountById(accountId)
	if err != nil {
		return nil, err
	}
	client := account.Get115Client()
	helpers.AppLogger.Infof("开始获取115目录列表, 父目录ID: %s", parentId)
	ctx := context.Background()
	resp, err := client.GetFsList(ctx, parentId, true, true, true, 0, 200)
	if err != nil {
		helpers.AppLogger.Warnf("获取115目录列表失败: 父目录：%s, 错误:%v", parentId, err)
		return nil, err
	}
	helpers.AppLogger.Infof("成功获取115目录列表, 父目录ID: %s, 文件数量: %d", parentId, len(resp.Data))
	folders := make([]DirResp, 0)
	// 构建Path
	for _, item := range resp.Data {
		parentPath := resp.PathStr
		if parentPath == "" {
			parentPath = ""
		}
		helpers.AppLogger.Infof("遍历 %s 的115目录列表, 路径: %s", parentPath, item.FileName)
		if item.FileCategory == v115open.TypeDir {
			folders = append(folders, DirResp{
				Id:   item.FileId,
				Name: item.FileName,
				Path: filepath.ToSlash(filepath.Join(parentPath, item.FileName)),
			})
		}
	}
	return folders, nil
}

func GetBaiduPanPathList(parentId string, accountId uint) ([]DirResp, error) {
	// 获取115目录列表
	account, err := models.GetAccountById(accountId)
	if err != nil {
		return nil, err
	}
	client := account.GetBaiDuPanClient()
	ctx := context.Background()
	fileList, fileErr := client.GetFileList(ctx, parentId, 1, 1, 0, 1000)
	if fileErr != nil {
		helpers.AppLogger.Warnf("获取百度网盘目录列表失败: 父目录：%s, 错误:%v", parentId, fileErr)
		return nil, fileErr
	}
	// helpers.AppLogger.Infof("成功获取百度网盘文件列表, 父目录ID: %s, 文件数量: %d", parentId, len(resp.Data))
	items := make([]DirResp, 0)
	// 构建Path
	for _, item := range fileList {
		// 去掉item.Path开头的/
		item.Path = strings.TrimPrefix(item.Path, "/")
		items = append(items, DirResp{
			Id:   item.Path,
			Name: filepath.Base(item.Path),
			Path: item.Path,
		})
	}
	return items, nil
}

type FileItem struct {
	Id          string `json:"id"`
	IsDirectory bool   `json:"is_directory"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ModifiedAt  int64  `json:"modified_time"`
}

// 返回目录和文件列表
func GetNetFileList(c *gin.Context) {
	type dirListReq struct {
		ParentId  string `json:"parent_id" form:"path"`
		AccountId uint   `json:"account_id" form:"account_id"`
		Page      int    `json:"page" form:"page"`
		PageSize  int    `json:"page_size" form:"page_size"`
	}
	var req dirListReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 1150
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取账号信息失败: " + err.Error(), Data: nil})
		return
	}
	var list []*FileItem
	switch account.SourceType {
	case models.SourceTypeOpenList:
		list, err = getOpenlistDirs(req.ParentId, account, req.Page, req.PageSize)
	case models.SourceType115:
		list, err = get115Dirs(req.ParentId, account, req.Page, req.PageSize)
	case models.SourceTypeBaiduPan:
		list, err = getBaiduPanDirs(req.ParentId, account, req.Page, req.PageSize)
	default:
		// 报错
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未知的网盘类型", Data: nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取目录列表失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[[]*FileItem]{Code: Success, Message: "", Data: list})
}

func getOpenlistDirs(parentPath string, account *models.Account, page, pageSize int) ([]*FileItem, error) {
	// 去掉parentPath末尾的/
	parentPath = strings.TrimSuffix(parentPath, "/")
	parentPath = strings.TrimSuffix(parentPath, "\\")
	helpers.AppLogger.Infof("开始获取OpenList目录列表, 父目录路径: %s", parentPath)
	client := account.GetOpenListClient()
	resp, err := client.FileList(context.Background(), parentPath, page, pageSize)
	if err != nil {
		return nil, err
	}
	// 只返回文件夹列表
	items := make([]*FileItem, 0)
	for _, item := range resp.Content {
		t, err := time.Parse(time.RFC3339, item.Modified)
		var mtime int64
		if err != nil {
			mtime = 0 // 错误时使用默认值
		} else {
			mtime = t.Unix() // 转换为Unix时间戳（秒）
		}
		items = append(items, &FileItem{
			Id:          parentPath + "/" + item.Name,
			IsDirectory: item.IsDir,
			Name:        item.Name,
			Size:        item.Size,
			ModifiedAt:  mtime,
		})
	}
	return items, nil
}

func get115Dirs(parentId string, account *models.Account, page, pageSize int) ([]*FileItem, error) {
	client := account.Get115Client()
	ctx := context.Background()
	if parentId == "" {
		parentId = "0"
	}
	resp, err := client.GetFsList(ctx, parentId, true, false, true, (page-1)*pageSize, pageSize)
	if err != nil {
		helpers.AppLogger.Warnf("获取115目录列表失败: 父目录：%s, 错误:%v", parentId, err)
		return nil, err
	}
	helpers.AppLogger.Infof("成功获取115文件列表, 父目录ID: %s, 文件数量: %d", parentId, len(resp.Data))
	items := make([]*FileItem, 0)
	// 构建Path
	for _, item := range resp.Data {
		items = append(items, &FileItem{
			Id:          item.FileId,
			IsDirectory: item.FileCategory == v115open.TypeDir,
			Name:        item.FileName,
			Size:        item.FileSize,
			ModifiedAt:  item.Ptime,
		})
	}
	return items, nil

}

func getBaiduPanDirs(parentId string, account *models.Account, page, pageSize int) ([]*FileItem, error) {
	client := account.GetBaiDuPanClient()
	ctx := context.Background()
	fileList, fileErr := client.GetFileList(ctx, parentId, 0, 1, int32((page-1)*pageSize), int32(pageSize))
	if fileErr != nil {
		helpers.AppLogger.Warnf("获取百度网盘目录列表失败: 父目录：%s, 错误:%v", parentId, fileErr)
		return nil, fileErr
	}
	// helpers.AppLogger.Infof("成功获取百度网盘文件列表, 父目录ID: %s, 文件数量: %d", parentId, len(resp.Data))
	items := make([]*FileItem, 0)
	// 构建Path
	for _, item := range fileList {
		items = append(items, &FileItem{
			Id:          item.Path,
			IsDirectory: item.IsDir == 1,
			Name:        filepath.Base(item.Path),
			Size:        int64(item.Size),
			ModifiedAt:  int64(item.ServerMtime),
		})
	}
	return items, nil
}

// 创建文件夹
func CreateDir(c *gin.Context) {
	type createDirReq struct {
		ParentId   string            `json:"parent_id" form:"parent_id"`
		ParentPath string            `json:"parent_path" form:"parent_path"`
		SourceType models.SourceType `json:"source_type" form:"source_type"`
		AccountId  uint              `json:"account_id" form:"account_id"`
		Name       string            `json:"name" form:"name"`
	}
	var req createDirReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "文件夹名称不能为空", Data: nil})
		return
	}
	// 父目录不能为空
	// if req.ParentPath == "" || req.ParentId == "" {
	// 	c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "父目录不能为空", Data: nil})
	// 	return
	// }
	// 文件夹名称不能包含/
	if strings.Contains(req.Name, "/") {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "文件夹名称不能包含/", Data: nil})
		return
	}
	var err error
	var pathId string
	switch req.SourceType {
	case models.SourceTypeLocal:
		pathId, err = makeLocalPath(req.ParentId, req.Name)
	case models.SourceTypeOpenList:
		pathId, err = makeOpenListPath(req.ParentId, req.Name, req.AccountId)
	case models.SourceType115:
		pathId, err = make115PathList(req.ParentId, req.ParentPath, req.Name, req.AccountId)
	case models.SourceTypeBaiduPan:
		pathId, err = makeBaiduPanPathList(req.ParentId, req.Name, req.AccountId)
	default:
		// 报错
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未知的同步源类型", Data: nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "创建目录失败: " + err.Error(), Data: nil})
		return
	}
	dirResp := DirResp{
		Id:   pathId,
		Name: req.Name,
		Path: filepath.ToSlash(filepath.Join(req.ParentPath, req.Name)),
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "创建目录成功", Data: dirResp})
}

// 创建本地目录
func makeLocalPath(parentId string, folderName string) (string, error) {
	// 检查父目录是否存在
	if !helpers.PathExists(parentId) || parentId == "" {
		return "", fmt.Errorf("父目录不存在: %s", parentId)
	}
	// 构建新目录路径
	newDir := filepath.Join(parentId, folderName)
	// 创建目录
	if err := os.Mkdir(newDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %s, 错误: %v", newDir, err)
	}
	return newDir, nil
}

// 创建openlist目录
func makeOpenListPath(parentId string, folderName string, accountId uint) (string, error) {
	if parentId == "" {
		parentId = "/"
	}
	// 检查父目录是否存在
	account, err := models.GetAccountById(accountId)
	if err != nil {
		return "", fmt.Errorf("获取账号失败: %v", err)
	}
	client := account.GetOpenListClient()
	_, err = client.FileDetail(parentId)
	if err != nil {
		return "", fmt.Errorf("获取openlist目录详情失败，目录可能不存在: %v", err)
	}
	newDir := filepath.ToSlash(filepath.Join(parentId, folderName))
	err = client.Mkdir(newDir)
	if err != nil {
		return "", fmt.Errorf("创建openlist目录失败: %s, 错误: %v", newDir, err)
	}
	return newDir, nil
}

// 创建115目录
func make115PathList(parentId, parentPath, folderName string, accountId uint) (string, error) {
	if parentId == "" {
		parentId = "0"
	}
	// 检查父目录是否存在
	account, err := models.GetAccountById(accountId)
	if err != nil {
		return "", fmt.Errorf("获取账号失败: %v", err)
	}
	client := account.Get115Client()
	if parentId != "0" {
		_, err = client.GetFsDetailByCid(context.Background(), parentId)
		if err != nil {
			return "", fmt.Errorf("获取115目录详情失败，目录可能不存在: %v", err)
		}
	}
	newDir := filepath.ToSlash(filepath.Join(parentPath, folderName))
	newPathId, err := client.MkDir(context.Background(), parentId, folderName)
	if err != nil {
		return "", fmt.Errorf("创建115目录失败: %s, 错误: %v", newDir, err)
	}
	return newPathId, nil
}

func makeBaiduPanPathList(parentId string, folderName string, accountId uint) (string, error) {
	if parentId == "" {
		parentId = "/"
	}
	// 检查父目录是否存在
	account, err := models.GetAccountById(accountId)
	if err != nil {
		return "", fmt.Errorf("获取账号失败: %v", err)
	}
	client := account.GetBaiDuPanClient()
	exists, err := client.PathExists(context.Background(), parentId)
	if err != nil {
		return "", fmt.Errorf("获取百度网盘目录失败，目录可能不存在: %v", err)
	}
	if !exists {
		return "", fmt.Errorf("父目录不存在: %s", parentId)
	}
	// 创建新目录
	newDir := filepath.ToSlash(filepath.Join(parentId, folderName))
	err = client.Mkdir(context.Background(), newDir)
	if err != nil {
		return "", fmt.Errorf("创建百度网盘目录失败: %s, 错误: %v", newDir, err)
	}
	return newDir, nil
}

// 更新飞牛有权限的目录
// 飞牛执行目录授权操作后，会触发该接口调用
func UpdateFNPath(c *gin.Context) {
	type updateFNPathReq struct {
		Path string `json:"path" form:"path"`
	}
	var req updateFNPathReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	helpers.AccessiblePathes = req.Path
	helpers.AppLogger.Infof("更新飞牛有权限的目录为: %s", req.Path)
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新目录成功", Data: nil})
}

func DeleteDir(c *gin.Context) {
	type deleteDirReq struct {
		ParentId  string `json:"parent_id" form:"parent_id"`
		FileId    string `json:"file_id" form:"file_id"`
		AccountId uint   `json:"account_id" form:"account_id"`
	}
	var req deleteDirReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	if req.FileId == "" || req.AccountId == 0 || req.FileId == "0" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取账号失败: " + err.Error(), Data: nil})
		return
	}
	switch account.SourceType {
	case models.SourceType115:
		client := account.Get115Client()
		_, err = client.Del(context.Background(), []string{req.FileId}, req.ParentId)
	case models.SourceTypeBaiduPan:
		client := account.GetBaiDuPanClient()
		err = client.Del(context.Background(), []string{req.FileId})
	default:
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "不支持的文件系统", Data: nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "删除目录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除目录成功", Data: nil})
}
