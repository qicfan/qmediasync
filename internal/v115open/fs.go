package v115open

import (
	"Q115-STRM/internal/helpers"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type FileType string

const (
	TypeFile FileType = "1"
	TypeDir  FileType = "0"
)

type FileParentPath struct {
	Name   string      `json:"name"` //父目录名
	FileId json.Number `json:"cid"`
}

type FileDetailPath struct {
	FileId string `json:"file_id"`
	Name   string `json:"file_name"`
}

type File struct {
	FileId       string      `json:"fid"`  // 文件ID
	Aid          string      `json:"aid"`  // 文件的状态，aid 的别名。1 正常，7 删除(回收站)，120 彻底删除
	Pid          string      `json:"pid"`  // 父文件夹ID
	FileCategory FileType    `json:"fc"`   // 文件类型 0 文件夹 1 文件
	FileName     string      `json:"fn"`   // 文件名
	PickCode     string      `json:"pc"`   // 文件提取码
	Utime        int64       `json:"upt"`  // 修改时间
	Ptime        int64       `json:"uppt"` // 上传时间
	Sha1         string      `json:"sha1"` // 文件sha1
	FileSize     int64       `json:"fs"`   // 文件大小
	Fta          string      `json:"fta"`  // 文件状态 0/2 未上传完成，1 已上传完成
	Ico          string      `json:"ico"`  // 文件后缀名
	VImg         string      `json:"v_img"`
	Thumbnail    string      `json:"thumb"`     // 图片缩略图
	Uo           string      `json:"uo"`        // 原图地址
	PlayLong     json.Number `json:"play_long"` // 视频时长,-1正在统计，其他是时长，单位秒
}

type FileListResp struct {
	RespBaseBool[[]File]
	Count    int              `json:"count"`
	SysCount int              `json:"sys_count"`
	Limit    json.Number      `json:"limit"`
	Offset   json.Number      `json:"offset"`
	Path     []FileParentPath `json:"path"`
	PathStr  string           `json:"path_str"` // 完整路径字符串
}

type FileDetail struct {
	Count        json.Number      `json:"count"`          // 包含文件总数量
	FileSize     string           `json:"size"`           // 文件夹或文件大小，字符串如："10GB"
	FileSizeByte int64            `json:"size_byte"`      // 大小的字节表示
	FolderCount  json.Number      `json:"folder_number"`  // 子文件夹的数量
	PlayLong     json.Number      `json:"play_long"`      // 视频时长,-1正在统计，其他是时长，单位秒
	ShowPlayLong json.Number      `json:"show_play_long"` // 是否展示视频时长
	Ptime        string           `json:"ptime"`          // 上传时间
	Utime        string           `json:"utime"`          // 修改时间
	FileName     string           `json:"file_name"`      // 文件名称
	PickCode     string           `json:"pick_code"`      // 提取码
	Sha1         string           `json:"sha1"`           // 文件的sha1值，秒传用
	FileId       string           `json:"file_id"`        // 文件ID
	OpenTime     json.Number      `json:"open_time"`      // 文件最近打开时间
	IsMark       string           `json:"is_mark"`        // 是否星标
	FileCategory FileType         `json:"file_category"`  // 类型，1-文件，0-文件夹
	Paths        []FileDetailPath `json:"paths"`          // 父目录
	Path         string           `json:"path"`           // 完整路径
}

type MkDirData struct {
	FileName string `json:"file_name"`
	FileId   string `json:"file_id"`
}

func (d *FileDetail) GetFullPath() string {
	// 生成完整路径
	baseDir := make([]string, 0, len(d.Paths))
	for _, item := range d.Paths {
		if item.FileId == "0" {
			baseDir = append(baseDir, "")
		} else {
			baseDir = append(baseDir, item.Name)
		}
	}
	return strings.Join(baseDir, "/")
}

// 查询文件（夹）列表
// 要查询的父目录CID，0是根目录
// showCur bool true-只显示当前目录下的列表，false-查询当前目录以及子目录内的所有列表
// offset 和 limit 搭配实现分页，limit最大1150
func (c *OpenClient) GetFsList(ctx context.Context, fileId string, showCur bool, onlyDir bool, showDir bool, offset int, limit int) (*FileListResp, error) {
	data := make(map[string]string)
	data["cid"] = fileId
	if limit != 0 {
		data["limit"] = helpers.IntToString(limit)
	}
	if offset != 0 {
		data["offset"] = helpers.IntToString(offset)
	}
	// 是否只显示当前目录下的列表
	if showCur {
		data["cur"] = "1"
	}
	// 是否包含文件夹
	if onlyDir {
		data["stdir"] = "1"
	}
	// 是否只显示文件夹
	if showDir {
		data["show_dir"] = "1"
	}
	url := fmt.Sprintf("%s/open/ufile/files", OPEN_BASE_URL)
	req := c.client.R().SetQueryParams(data).SetMethod("GET")
	_, respBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(3, 1, 15), nil)
	if err != nil {
		helpers.V115Log.Errorf("调用115文件列表接口失败: %v", err)
		return nil, err
	}
	// 解码
	respData := FileListResp{}
	jsonErr := json.Unmarshal(respBytes, &respData)
	if jsonErr != nil {
		helpers.V115Log.Errorf("解析文件列表接口响应失败: %v", jsonErr)
		return nil, jsonErr
	}
	// 生成路径字符串
	pathStr := make([]string, 0, len(respData.Path))
	for _, item := range respData.Path {
		if item.FileId == "0" {
			continue
		}
		pathStr = append(pathStr, item.Name)
	}
	respData.PathStr = filepath.ToSlash(filepath.Join(pathStr...))
	pathStr = nil
	return &respData, nil
}

// 根据路径查询详情
// POST 域名 + /open/folder/get_info
func (c *OpenClient) GetFsDetailByPath(ctx context.Context, path string) (*FileDetail, error) {
	if path == "." || path == "/" || path == "" {
		return nil, fmt.Errorf("不能查询根目录的详情")
	}
	// 将路径中所有\替换为/
	path = strings.ReplaceAll(path, "\\", "/")
	if !strings.HasPrefix(path, "/") && path != "." && path != "" {
		path = "/" + path
	}
	data := make(map[string]string)
	data["path"] = path
	helpers.V115Log.Infof("调用文件详情接口, path: %s", path)
	url := fmt.Sprintf("%s/open/folder/get_info", OPEN_BASE_URL)
	req := c.client.R().SetFormData(data).SetMethod("POST")
	var respData *FileDetail = &FileDetail{}
	_, bodyBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(3, 1, 60), respData)
	if err != nil {
		helpers.V115Log.Errorf("调用文件详情接口失败: %v", err)
		return nil, err
	}
	resp := &RespBaseBool[json.RawMessage]{}
	bodyErr := json.Unmarshal(bodyBytes, &resp)
	if bodyErr != nil {
		helpers.V115Log.Errorf("解析文件详情接口响应失败: %v", bodyErr)
		return respData, bodyErr
	}
	if resp.Code != 0 {
		// helpers.V115Log.Errorf("文件 %s 不存在: %v", fileId, err)
		return nil, fmt.Errorf("Code=%d, Message=%s", resp.Code, resp.Message)
	}
	if err != nil {
		helpers.V115Log.Errorf("调用文件详情接口失败: %v", err)
		return nil, err
	}
	if respData.FileId == "" {
		return nil, fmt.Errorf("115 返回空数据")
	}
	return respData, nil
}

// 根据CID查询详情
// GET 域名 + /open/folder/get_info
func (c *OpenClient) GetFsDetailByCid(ctx context.Context, fileId string) (*FileDetail, error) {
	if fileId == "" {
		return nil, fmt.Errorf("fileId is empty")
	}
	data := make(map[string]string)
	data["file_id"] = fileId
	url := fmt.Sprintf("%s/open/folder/get_info", OPEN_BASE_URL)
	req := c.client.R().SetQueryParams(data).SetMethod("GET")
	var respData *FileDetail = &FileDetail{}
	_, bodyBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(3, 1, 60), respData)
	resp := &RespBaseBool[json.RawMessage]{}
	// helpers.V115Log.Debugf("调用文件详情接口, fileId: %s => %s", fileId, string(bodyBytes))
	bodyErr := json.Unmarshal(bodyBytes, &resp)
	if bodyErr != nil {
		helpers.V115Log.Errorf("解析文件详情接口响应失败: %s => %v", string(bodyBytes), bodyErr)
		return respData, bodyErr
	}
	if resp.Code != 0 {
		// helpers.V115Log.Errorf("文件 %s 不存在: %v", fileId, err)
		return nil, fmt.Errorf("Code=%d, Message=%s", resp.Code, resp.Message)
	}
	if err != nil {
		helpers.V115Log.Errorf("调用文件详情接口失败: %v", err)
		return nil, err
	}
	if respData.FileId == "" {
		return nil, fmt.Errorf("115 返回空数据")
	}
	// helpers.AppLogger.Infof("文件 %s 详情中的文件名: %s", fileId, respData.FileName)
	return respData, nil
}

// 重命名
// Path： POST 域名 + /open/ufile/update
func (c *OpenClient) ReName(ctx context.Context, fileId string, newName string) (bool, error) {
	data := make(map[string]string)
	data["file_id"] = fileId
	data["file_name"] = newName
	url := fmt.Sprintf("%s/open/open/ufile/update", OPEN_BASE_URL)
	req := c.client.R().SetFormData(data).SetMethod("POST")
	respData := RespBaseBool[interface{}]{}
	_, respBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(0, 0, 0), nil)
	if err != nil {
		helpers.V115Log.Errorf("调用文件更新接口失败: %v", err)
		return false, err
	}
	jsonErr := json.Unmarshal(respBytes, &respData)
	if jsonErr != nil || !respData.State {
		helpers.V115Log.Errorf("重命名%s => %s: %v", fileId, newName, jsonErr)
		return false, jsonErr
	}

	return respData.State, nil
}

// 批量移动文件（夹）
// POST 域名 + /open/ufile/move
// 多个文件用半角逗号分隔
func (c *OpenClient) Move(ctx context.Context, fileIds []string, toFileId string) (bool, error) {
	data := make(map[string]string)
	data["file_ids"] = strings.Join(fileIds, ",")
	data["to_cid"] = toFileId
	url := fmt.Sprintf("%s/open/open/ufile/move", OPEN_BASE_URL)
	req := c.client.R().SetFormData(data).SetMethod("POST")
	respData := RespBaseBool[interface{}]{}
	_, respBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(0, 0, 0), nil)
	if err != nil {
		helpers.V115Log.Errorf("调用文件移动接口失败: %v", err)
		return false, err
	}
	jsonErr := json.Unmarshal(respBytes, &respData)
	if jsonErr != nil || !respData.State {
		helpers.V115Log.Errorf("移动文件失败%+v=>%s: %v", fileIds, toFileId, jsonErr)
		return false, jsonErr
	}
	return respData.State, nil
}

// 批量复制文件（夹）
// POST 域名 + /open/ufile/copy
// 多个文件用半角逗号分隔
func (c *OpenClient) Copy(ctx context.Context, fileIds []string, toFileId string, overwrite bool) (bool, error) {
	data := make(map[string]string)
	data["file_id"] = strings.Join(fileIds, ",")
	data["pid"] = toFileId
	if overwrite {
		data["nodupli"] = "0"
	} else {
		data["nodupli"] = "1"
	}
	url := fmt.Sprintf("%s/open/open/ufile/copy", OPEN_BASE_URL)
	req := c.client.R().SetFormData(data).SetMethod("POST")
	respData := RespBaseBool[interface{}]{}
	_, respBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(0, 0, 0), nil)
	if err != nil {
		helpers.V115Log.Errorf("调用文件复制接口失败: %v", err)
		return false, err
	}
	jsonErr := json.Unmarshal(respBytes, &respData)
	if jsonErr != nil || !respData.State {
		helpers.V115Log.Errorf("复制文件失败%+v=>%s: %v", fileIds, toFileId, jsonErr)
		return false, jsonErr
	}
	return respData.State, nil
}

// 批量删除文件（夹）
// POST 域名 + /open/ufile/delete
// 多个文件用半角逗号分隔
func (c *OpenClient) Del(ctx context.Context, fileIds []string, parentFileId string) (bool, error) {
	data := make(map[string]string)
	data["file_ids"] = strings.Join(fileIds, ",")
	if parentFileId != "" {
		data["parent_id"] = parentFileId
	}
	url := fmt.Sprintf("%s/open/open/ufile/delete", OPEN_BASE_URL)
	req := c.client.R().SetFormData(data).SetMethod("POST")
	respData := RespBaseBool[interface{}]{}
	_, respBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(0, 0, 0), nil)
	if err != nil {
		helpers.V115Log.Errorf("调用文件删除接口失败: %v", err)
		return false, err
	}
	jsonErr := json.Unmarshal(respBytes, &respData)
	if jsonErr != nil || !respData.State {
		helpers.V115Log.Errorf("删除文件失败%+v=>%s: %v", fileIds, parentFileId, jsonErr)
		return false, jsonErr
	}
	return respData.State, nil
}

// 新建文件夹
// POST 域名 + /open/folder/add
func (c *OpenClient) MkDir(ctx context.Context, parentFileId string, fileName string) (string, error) {
	data := make(map[string]string)
	data["file_name"] = fileName
	data["pid"] = parentFileId
	url := fmt.Sprintf("%s/open/open/folder/add", OPEN_BASE_URL)
	req := c.client.R().SetFormData(data).SetMethod("POST")
	respData := &MkDirData{}
	_, respBytes, err := c.doAuthRequest(ctx, url, req, MakeRequestConfig(0, 0, 0), respData)
	if err != nil {
		helpers.V115Log.Errorf("调用新建文件夹接口失败: %v", err)
		return "", err
	}
	resp := RespBaseBool[json.RawMessage]{}
	jsonErr := json.Unmarshal(respBytes, &resp)
	if jsonErr != nil || !resp.State {
		helpers.V115Log.Errorf("新建文件夹失败: %v", jsonErr)
		return "", jsonErr
	}
	return respData.FileId, nil
}
