package syncstrm

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type StrmData struct {
	UserId   string `json:"userid"`    // 用户ID
	PickCode string `json:"pick_code"` // 文件ID
	Sign     string `json:"sign"`      // 文件签名
	Path     string `json:"path"`      // 115的路径
	BaseUrl  string `json:"base_url"`  // 115的base_url
	UrlPath  string `json:"url_path"`  // 115的url_path
}

// 生成strm文件
// st只能是来源路径，所以需要生成strm文件的路径
func (s *SyncStrm) ProcessStrmFile(sf *SyncFileCache) error {
	rs := s.CompareStrm(sf)
	if rs == 1 {
		// s.Sync.Logger.Infof("文件 %s 已存在且无需更新strm文件，跳过", filepath.Join(sf.Path, sf.FileName))
		return nil
	}
	// localFilePath := sf.GetLocalFilePath()
	strmFullPath := sf.GetLocalFilePath(s.TargetPath, s.SourcePath)
	strmContent := s.SyncDriver.MakeStrmContent(sf)
	// 写入文件并设置所有者
	err := helpers.WriteFileWithPerm(strmFullPath, []byte(strmContent), 0777)
	if err != nil {
		s.Sync.Logger.Errorf("写入strm文件并设置所有者失败: %v", err)
		return err
	}
	// 修改文件时间
	if sf.MTime > 0 {
		err := os.Chtimes(strmFullPath, time.Unix(sf.MTime, 0), time.Unix(sf.MTime, 0))
		if err != nil {
			s.Sync.Logger.Errorf("修改strm文件时间失败: %v", err)
			return err
		}
	}
	s.Sync.Logger.Infof("[生成strm] %s => %s", strmFullPath, strmContent)
	atomic.AddInt64(&s.NewStrm, 1)
	return nil
}

// 1-无需操作，0-更新
func (s *SyncStrm) CompareStrm(st *SyncFileCache) int {
	localFilePath := st.GetLocalFilePath(s.TargetPath, s.SourcePath)
	if !helpers.PathExists(localFilePath) {
		// s.Sync.Logger.Infof("文件 %s 不存在，需要生成strm文件", st.LocalFilePath)
		return 0
	}
	if st.SourceType == models.SourceTypeLocal {
		// s.Sync.Logger.Infof("文件 %s 来源本地，不需要生成strm文件", filepath.Join(st.Path, st.FileName))
		return 1
	}
	// 读取strm文件内容
	strmData := s.LoadDataFromStrm(localFilePath)
	if strmData == nil {
		return 0
	}
	if st.SourceType == models.SourceTypeOpenList {
		account, err := models.GetAccountById(s.Account.ID)
		if err != nil {
			s.Sync.Logger.Errorf("获取Openlist账号信息失败: %v", err)
			return 0
		}
		// 比较主机名称是否相同
		if strmData.BaseUrl != account.BaseUrl {
			s.Sync.Logger.Warnf("文件 %s 的STRM内容的主机名称与本地不一致, 本地: %s, 远程: %s", filepath.Join(st.Path, st.FileName), account.BaseUrl, strmData.BaseUrl)
			return 0
		}
		if strmData.Sign != st.OpenlistSign {
			s.Sync.Logger.Warnf("文件 %s 的STRM内容的签名参数与本地不一致, 本地: %s, 远程: %s", filepath.Join(st.Path, st.FileName), st.OpenlistSign, strmData.Sign)
			return 0
		}
	}
	if st.SourceType == models.SourceType115 || st.SourceType == models.SourceTypeBaiduPan {
		// 比较路径是否相同
		if s.Config.StrmUrlNeedPath == 1 {
			stPath := filepath.ToSlash(filepath.Join(st.Path, st.FileName))
			if strmData.Path != stPath {
				s.Sync.Logger.Warnf("文件 %s 的STRM内容的路径与本地不一致, 本地: %s, 远程: %s", stPath, stPath, strmData.Path)
				return 0
			}
		} else {
			if strmData.Path != "" {
				s.Sync.Logger.Warnf("文件 %s 的STRM内容的含有完整路径 %s，但是设置中关闭了添加路径，所以重新生成strm以去掉路径s", filepath.Join(st.Path, st.FileName), strmData.Path)
				return 0
			}
		}
		// 比较主机名称是否相同
		// 如果StrmBaseUrl以/结尾，那删除掉末尾的/
		if before, ok := strings.CutSuffix(s.Config.StrmBaseUrl, "/"); ok {
			s.Config.StrmBaseUrl = before
		}
		if strmData.BaseUrl != s.Config.StrmBaseUrl {
			s.Sync.Logger.Warnf("文件 %s 的STRM内容的主机名称与本地不一致, 本地: %s, 远程: %s", filepath.Join(st.Path, st.FileName), s.Config.StrmBaseUrl, strmData.BaseUrl)
			return 0
		}
		// 如果没有PickCode，则更新以补全
		if strmData.PickCode == "" {
			s.Sync.Logger.Warnf("文件 %s 的STRM内容缺少PickCode: %s, 补全", filepath.Join(st.Path, st.FileName), strmData.PickCode)
			return 0
		}
		if strmData.UserId != s.Account.UserId {
			s.Sync.Logger.Warnf("文件 %s 的STRM内容的用户ID与本地不一致, 本地: %s, 远程: %s", filepath.Join(st.Path, st.FileName), s.Account.UserId, strmData.UserId)
			return 0
		}
		// 比较UrlPath，如果是iso文件，UrlPath必须以.iso结尾
		ext := filepath.Ext(st.FileName)
		if !strings.HasSuffix(strmData.UrlPath, ext) {
			s.Sync.Logger.Warnf("文件 %s 的STRM内容的Url路径 %s 没有以 %s 结尾，重新生成", filepath.Join(st.Path, st.FileName), strmData.UrlPath, ext)
			return 0
		}
	}
	return 1
}

// 解析strm文件内url的参数并返回
func (s *SyncStrm) LoadDataFromStrm(strmPath string) *StrmData {
	if !helpers.PathExists(strmPath) {
		// s.Sync.Logger.Errorf("strm文件不存在: %s", strmPath)
		return nil
	}
	data, err := os.ReadFile(strmPath)
	if err != nil {
		s.Sync.Logger.Errorf("读取strm文件失败: %v", err)
		return nil
	}
	var strmData StrmData
	strmUrl, urlErr := url.Parse(string(data))
	if urlErr != nil {
		s.Sync.Logger.Errorf("解析strm文件失败: %v", urlErr)
		return nil
	}
	strmData.UrlPath = strmUrl.Path
	queryParams := strmUrl.Query()
	if pickCode := queryParams.Get("pickcode"); pickCode != "" {
		strmData.PickCode = pickCode
	}
	if userId := queryParams.Get("userid"); userId != "" {
		strmData.UserId = userId
	}
	strmData.Sign = ""
	if sign := queryParams.Get("sign"); sign != "" {
		strmData.Sign = sign
	}
	strmData.Path = ""
	if path := queryParams.Get("path"); path != "" {
		strmData.Path = path
	}
	strmData.BaseUrl = fmt.Sprintf("%s://%s", strmUrl.Scheme, strmUrl.Host)
	return &strmData
}

func (sf *SyncStrm) GetRemoteFilePathUrlEncode(path string) string {
	// 中文保留，只对特殊字符编码
	path = strings.ReplaceAll(path, "/", "%2F")
	path = strings.ReplaceAll(path, "?", "%3F")
	path = strings.ReplaceAll(path, "&", "%26")
	path = strings.ReplaceAll(path, "=", "%3D")
	path = strings.ReplaceAll(path, "+", "%2B")
	path = strings.ReplaceAll(path, "#", "%23")
	path = strings.ReplaceAll(path, "@", "%40")
	path = strings.ReplaceAll(path, "!", "%21")
	path = strings.ReplaceAll(path, "$", "%24")
	path = strings.ReplaceAll(path, " ", "%20")
	path = strings.ReplaceAll(path, "%", "%25")
	return path
}
