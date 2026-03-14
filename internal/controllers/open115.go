package controllers

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type v115StatusResp struct {
	UserId      json.Number `json:"user_id"`
	Username    string      `json:"username"`
	UsedSpace   int64       `json:"used_space"`
	TotalSpace  int64       `json:"total_space"`
	MemberLevel string      `json:"member_level"`
	ExpireTime  string      `json:"expire_time"`
}

type KeyLockWithTimeout struct {
	mutexes sync.Map // key -> *sync.Mutex
	global  sync.Mutex
}

// LockWithTimeout 尝试获取锁，如果超时则返回 false
func (kl *KeyLockWithTimeout) LockWithTimeout(key string, timeout time.Duration) bool {
	kl.global.Lock()
	mutex, _ := kl.mutexes.LoadOrStore(key, &sync.Mutex{})
	kl.global.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false // 超时
		default:
			if mutex.(*sync.Mutex).TryLock() {
				return true // 成功获取锁
			}
			time.Sleep(10 * time.Millisecond) // 短暂等待后重试
		}
	}
}

func (kl *KeyLockWithTimeout) Unlock(key string) {
	kl.global.Lock()
	mutex, ok := kl.mutexes.Load(key)
	kl.global.Unlock()

	if ok {
		mutex.(*sync.Mutex).Unlock()
	}
}

// Get115Status 查询115账号状态
// @Summary 查询115账号状态
// @Description 获取指定115账号的登录状态及存储信息
// @Tags 115开放平台
// @Accept json
// @Produce json
// @Param account_id query integer true "账号ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /auth/115-status [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func Get115Status(c *gin.Context) {
	type statusReq struct {
		AccountId uint `json:"account_id" form:"account_id"`
	}
	var req statusReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	client := account.Get115Client()
	var resp v115StatusResp
	// 获取用户信息
	userInfo, err := client.UserInfo()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115用户信息失败: " + err.Error(), Data: nil})
		return
	}
	resp.UserId = userInfo.UserId
	resp.Username = userInfo.UserName
	resp.UsedSpace = userInfo.RtSpaceInfo.AllUse.Size
	resp.TotalSpace = userInfo.RtSpaceInfo.AllTotal.Size
	resp.MemberLevel = userInfo.VipInfo.LevelName
	if userInfo.VipInfo.Expire > 0 {
		resp.ExpireTime = helpers.FormatTimestamp(userInfo.VipInfo.Expire)
	} else {
		resp.ExpireTime = "未开通会员"
	}
	// 返回状态信息
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取115状态成功", Data: resp})
}

func GetFileDetail(c *gin.Context) {
	type fileDetailReq struct {
		AccountId uint   `json:"account_id" form:"account_id"`
		FileId    string `json:"file_id" form:"file_id"`
	}
	var req fileDetailReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	fullPath := models.GetPathByPathFileId(account, req.FileId)
	if fullPath == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "文件ID不存在或未找到对应路径", Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取文件详情成功", Data: fullPath})
}

var keyLock KeyLockWithTimeout

// checkURLValidity 使用HEAD请求检查URL是否有效
// 返回true表示URL有效（2xx状态码），false表示URL已失效
func checkURLValidity(urlStr string) bool {
	client := &http.Client{
		Timeout: 3 * time.Second, // 3秒超时
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 不跟随重定向，只检查第一次响应
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		helpers.AppLogger.Warnf("创建HEAD请求失败: %v", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		helpers.AppLogger.Warnf("HEAD请求失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	// 检查状态码是否为2xx
	isValid := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !isValid {
		helpers.AppLogger.Warnf("URL已失效，状态码: %d, URL: %s", resp.StatusCode, urlStr)
	}
	return isValid
}

// 查询并302跳转到115文件直链
// 请求下载链接的user-agent必须跟访问下载链接的user-agent相同
func Get115FileUrl(c *gin.Context) {
	type fileUrlReq struct {
		AccountId uint   `json:"account_id" form:"account_id"`
		PickCode  string `json:"pick_code" form:"pick_code"`
		Force     int    `json:"force" form:"force"`
	}

	ua := c.Request.UserAgent()
	var req fileUrlReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	if req.PickCode == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "pick_code 参数不能为空", Data: nil})
		return
	}
	helpers.AppLogger.Infof("检查是否具有302播放标记， force=%d", req.Force)
	cacheKey := fmt.Sprintf("115url:%s, ua=%s", req.PickCode, ua)
	helpers.AppLogger.Infof("准备获取115文件下载链接: pickcode=%s, ua=%s，8095播放=%d 加锁10秒", req.PickCode, ua, req.Force)
	if keyLock.LockWithTimeout(cacheKey, 10*time.Second) {
		defer keyLock.Unlock(cacheKey)
		account, err := models.GetAccountById(req.AccountId)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
			return
		}
		client := account.Get115Client()

		// helpers.AppLogger.Debugf("是否启用本地代理：%d", models.SettingsGlobal.LocalProxy)
		if req.Force == 0 && models.SettingsGlobal.LocalProxy == 1 {
			// 跳转到本地代理时使用统一的UA
			ua = v115open.DEFAULTUA
			helpers.AppLogger.Infof("因为8095标识=%d, 本地播放代理开关=%d，所以使用默认UA: %s", req.Force, models.SettingsGlobal.LocalProxy, ua)
		}
		cachedUrl := string(db.Cache.Get(cacheKey))
		if cachedUrl == "" {
			cachedUrl = client.GetDownloadUrl(context.Background(), req.PickCode, ua, true)
			if cachedUrl == "" {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115下载链接失败", Data: nil})
				return
			}
			helpers.AppLogger.Infof("从接口中查询到115下载链接: pickcode=%s, ua=%s => %s", req.PickCode, ua, cachedUrl)
			// 缓存半小时
			db.Cache.Set(cacheKey, []byte(cachedUrl), 1800)
		} else {
			helpers.AppLogger.Infof("从缓存中查询到115下载链接: pickcode=%s, ua=%s => %s", req.PickCode, ua, cachedUrl)
			// 检查缓存链接是否有效
			if !checkURLValidity(cachedUrl) {
				helpers.AppLogger.Warnf("缓存链接已失效，删除缓存并重新获取: pickcode=%s, ua=%s", req.PickCode, ua)
				db.Cache.Delete(cacheKey)
				// 重新获取链接
				cachedUrl = client.GetDownloadUrl(context.Background(), req.PickCode, ua, true)
				if cachedUrl == "" {
					c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115下载链接失败", Data: nil})
					return
				}
				helpers.AppLogger.Infof("重新获取到115下载链接: pickcode=%s, ua=%s => %s", req.PickCode, ua, cachedUrl)
				// 缓存半小时
				db.Cache.Set(cacheKey, []byte(cachedUrl), 1800)
			}
		}
		if req.Force == 0 {
			if models.SettingsGlobal.LocalProxy == 1 {
				// 跳转到本地代理
				helpers.AppLogger.Infof("通过本地代理访问115下载链接，非302播放: %s", cachedUrl)
				proxyUrl := fmt.Sprintf("/proxy-115?url=%s", url.QueryEscape(cachedUrl))
				c.Redirect(http.StatusFound, proxyUrl)
			} else {
				helpers.AppLogger.Infof("302重定向到115下载链接，非302播放: %s", cachedUrl)
				c.Redirect(http.StatusFound, cachedUrl)
			}
		} else {
			helpers.AppLogger.Infof("302重定向到115下载链接， 302播放: %s", cachedUrl)
			c.Redirect(http.StatusFound, cachedUrl)
		}
	}
}

// Get115UrlByPickCode 查询115直链并重定向
// @Summary 获取115文件直链
// @Description 根据pickcode查询115文件直链并按需302跳转
// @Tags 115开放平台
// @Accept json
// @Produce json
// @Param pickcode query string true "文件PickCode"
// @Param userid query string false "115用户ID"
// @Param force query integer false "是否强制直链播放，1为直链，0使用本地代理时会走代理"
// @Success 302 {string} string "重定向到文件直链"
// @Failure 200 {object} object
// @Router /115/newurl [get]
func Get115UrlByPickCode(c *gin.Context) {
	type fileIdReq struct {
		UserId   string `json:"userid" form:"userid"`
		PickCode string `json:"pickcode" form:"pickcode"`
		Force    int    `json:"force" form:"force"`
	}
	var req fileIdReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	pickCode := req.PickCode
	userId := req.UserId
	var account *models.Account
	if userId == "" {
		// 查询SyncFile
		syncFile := models.GetFileByPickCode(pickCode)
		if syncFile == nil {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "文件PickCode不存在", Data: nil})
			return
		}
		var err error
		account, err = models.GetAccountById(syncFile.AccountId)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
			return
		}
		// helpers.AppLogger.Infof("通过PickCode查询到115账号: %s", account.Username)
	} else {
		var err error
		// 通过userId查询账号
		account, err = models.GetAccountByUserId(userId)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "用户ID不存在", Data: nil})
			return
		}
		// helpers.AppLogger.Infof("通过用户ID查询到115账号: %s", account.Username)
	}
	ua := c.Request.UserAgent()
	client := account.Get115Client()
	// helpers.AppLogger.Infof("检查是否具有直链播放标记， force=%d", req.Force)
	cacheKey := fmt.Sprintf("115url:%s, ua=%s", pickCode, ua)
	// helpers.AppLogger.Infof("准备获取115文件下载链接: pickcode=%s, ua=%s，8095播放=%d 加锁10秒", pickCode, ua, req.Force)
	if keyLock.LockWithTimeout(cacheKey, 10*time.Second) {
		defer keyLock.Unlock(cacheKey)
		// helpers.AppLogger.Debugf("是否启用本地代理：%d", models.SettingsGlobal.LocalProxy)
		if req.Force == 0 && models.SettingsGlobal.LocalProxy == 1 {
			// 跳转到本地代理时使用统一的UA
			ua = v115open.DEFAULTUA
			helpers.AppLogger.Infof("因为直链标识=%d, 本地播放代理开关=%d，所以使用默认UA: %s", req.Force, models.SettingsGlobal.LocalProxy, ua)
		}
		cachedUrl := string(db.Cache.Get(cacheKey))
		if cachedUrl == "" {
			cachedUrl = client.GetDownloadUrl(context.Background(), pickCode, ua, true)
			if cachedUrl == "" {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115下载链接失败", Data: nil})
				return
			}
			helpers.AppLogger.Infof("从接口中查询到115下载链接: pickcode=%s, ua=%s => %s", pickCode, ua, cachedUrl)
			// 缓存50分钟
			db.Cache.Set(cacheKey, []byte(cachedUrl), 3000)
		} else {
			helpers.AppLogger.Infof("从缓存中查询到115下载链接: pickcode=%s, ua=%s => %s", pickCode, ua, cachedUrl)
			// 检查缓存链接是否有效
			if !checkURLValidity(cachedUrl) {
				helpers.AppLogger.Warnf("缓存链接已失效，删除缓存并重新获取: pickcode=%s, ua=%s", pickCode, ua)
				db.Cache.Delete(cacheKey)
				// 重新获取链接
				cachedUrl = client.GetDownloadUrl(context.Background(), pickCode, ua, true)
				if cachedUrl == "" {
					c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115下载链接失败", Data: nil})
					return
				}
				helpers.AppLogger.Infof("重新获取到115下载链接: pickcode=%s, ua=%s => %s", pickCode, ua, cachedUrl)
				// 缓存50分钟
				db.Cache.Set(cacheKey, []byte(cachedUrl), 3000)
			}
		}
		if req.Force == 0 {
			if models.SettingsGlobal.LocalProxy == 1 {
				// 跳转到本地代理
				helpers.AppLogger.Infof("通过本地代理访问115下载链接，emby端口播放: %s", cachedUrl)
				proxyUrl := fmt.Sprintf("/proxy-115?url=%s", url.QueryEscape(cachedUrl))
				c.Redirect(http.StatusFound, proxyUrl)
			} else {
				helpers.AppLogger.Infof("302重定向到115下载链接，emby端口播放: %s", cachedUrl)
				c.Redirect(http.StatusFound, cachedUrl)
			}
		} else {
			helpers.AppLogger.Infof("302重定向到115下载链接， 直链播放: %s", cachedUrl)
			c.Redirect(http.StatusFound, cachedUrl)
		}
	}
}

// GetLoginQrCodeOpen 获取115开放平台登录二维码
// @Summary 获取115登录二维码
// @Description 生成115开放平台登录二维码并异步轮询状态
// @Tags 115开放平台
// @Accept json
// @Produce json
// @Param account_id body integer true "账号ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /auth/115-qrcode-open [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetLoginQrCodeOpen(c *gin.Context) {
	type qrcodeReq struct {
		AccountId uint `json:"account_id" form:"account_id"`
	}
	var req qrcodeReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	client := account.Get115Client()
	qrCodeData, err := client.GetQrCode()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取二维码失败: " + err.Error(), Data: nil})
		return
	}
	// 轮询二维码状态
	go func(codeData *v115open.QrCodeDataReturn) {
		for {
			status, err := client.QrCodeScanStatus(&codeData.QrCodeData)
			if err != nil {
				helpers.AppLogger.Errorf("刷新二维码状态失败: %v", err)
				break
			}
			// 写入缓存（假设有 db.Cache.Set 方法，缓存60秒）
			db.Cache.Set("qr_status:"+codeData.Uid, []byte(strconv.Itoa(int(status))), 60)
			if status == v115open.QrCodeScanStatusExpired {
				// 二维码已过期，重新获取
				helpers.AppLogger.Info("二维码已过期，重新获取")
				break
			}
			if status == v115open.QrCodeScanStatusConfirmed {
				// 二维码已确认，结束轮询，并且获取token
				helpers.AppLogger.Info("二维码已确认，结束轮询")
				openToken, err := client.GetToken(codeData)
				if err != nil || openToken == nil {
					c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取token失败: " + err.Error(), Data: nil})
					return
				}
				// 保存token
				account.UpdateToken(openToken.AccessToken, openToken.RefreshToken, openToken.ExpiresIn)
				userInfo, err := client.UserInfo()
				if err != nil {
					c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115用户信息失败: " + err.Error(), Data: nil})
					return
				}
				rs := account.UpdateUser(string(userInfo.UserId), userInfo.UserName)
				if !rs {
					c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新用户信息失败", Data: nil})
					return
				}
				break
			}
			// 每5秒刷新一次
			time.Sleep(5 * time.Second)
		}
	}(qrCodeData)
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取二维码成功", Data: qrCodeData})
}

// GetQrCodeStatus 查询二维码扫码状态
// @Summary 查询115二维码扫码状态
// @Description 查询指定二维码UID的扫码进度
// @Tags 115开放平台
// @Accept json
// @Produce json
// @Param uid body string true "二维码UID"
// @Param account_id body integer true "账号ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /auth/115-qrcode-status [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetQrCodeStatus(c *gin.Context) {
	type qrcodeReq struct {
		Uid       string `json:"uid" form:"uid"`
		AccountId uint   `json:"account_id" form:"account_id"`
	}
	var req qrcodeReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	uid := req.Uid
	s := v115open.QrCodeScanStatusNotScanned
	cachedStatus := db.Cache.Get("qr_status:" + uid)
	if cachedStatus != nil {
		statusInt, err := strconv.Atoi(string(cachedStatus))
		if err == nil {
			s = v115open.QrCodeScanStatus(statusInt)
		}
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "", Data: map[string]v115open.QrCodeScanStatus{"status": s}})
}

// 生成并返回115 oauth登录地址
func GetOAuthUrl(c *gin.Context) {
	accountId := c.Query("account_id")
	redirectUrl := c.Query("redirect_url")

	if accountId == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "缺少c账号ID参数", Data: nil})
		return
	}
	account, err := models.GetAccountById(uint(helpers.StringToInt(accountId)))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	clientId := account.AppId
	if clientId != "Q115-STRM" && clientId != "MQ的媒体库" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "自定义的APPID无法授权", Data: nil})
		return
	}
	baseUrl := helpers.GlobalConfig.AuthServer
	type stateData struct {
		State       string `json:"state"`
		Time        int64  `json:"time"`
		ClientId    string `json:"client_id"`
		RedirectUrl string `json:"redirect_url"`
		AccountId   uint   `json:"account_id"`
	}
	stateObj := stateData{
		State:       helpers.RandStr(16),
		Time:        time.Now().Unix(),
		ClientId:    clientId,
		RedirectUrl: fmt.Sprintf("%s?source=115", redirectUrl),
		AccountId:   account.ID,
	}
	// helpers.AppLogger.Infof("生成OAuth登录地址: %+v", stateObj)
	stateJson, _ := json.Marshal(stateObj)
	stateEncoded, err := helpers.Encrypt(string(stateJson))
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "生成OAuth登录地址失败: " + err.Error(), Data: nil})
		return
	}
	oauthUrl := fmt.Sprintf("%s/115.php?action=code&state=%s", baseUrl, stateEncoded)
	c.JSON(http.StatusOK, APIResponse[string]{Code: Success, Message: "获取115 OAuth登录地址成功", Data: oauthUrl})
}

func ConfirmOAuthCode(c *gin.Context) {
	type oauthReq struct {
		AccountId uint   `json:"account_id" form:"account_id"`
		Data      string `json:"data" form:"data"`
	}
	var req oauthReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	// 对req.Data解密
	decryptedData, err := helpers.Decrypt(req.Data)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "确认OAuth登录失败: " + err.Error(), Data: nil})
		return
	}
	type oauthData struct {
		State   json.Number `json:"state"`
		Code    json.Number `json:"code"`
		Message string      `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
		Error string      `json:"error"`
		Errno json.Number `json:"errno"`
	}
	var data oauthData
	err = json.Unmarshal([]byte(decryptedData), &data)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "确认OAuth登录失败: " + err.Error(), Data: nil})
		return
	}
	// 将token和刷新token保存到账号
	account.UpdateToken(data.Data.AccessToken, data.Data.RefreshToken, data.Data.ExpiresIn)
	// 更新用户名和用户ID
	client := account.Get115Client()
	userInfo, err := client.UserInfo()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取115用户信息失败: " + err.Error(), Data: nil})
		return
	}
	rs := account.UpdateUser(string(userInfo.UserId), userInfo.UserName)
	if !rs {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新用户信息失败", Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "确认OAuth登录成功", Data: nil})
}

// GetQueueStats 获取115 OpenAPI请求队列的统计数据
func GetQueueStats(c *gin.Context) {
	// 获取查询参数，支持查询不同时间窗口的统计
	timeWindowStr := c.DefaultQuery("time_window", "3600") // 默认3600秒（1小时）
	timeWindow, err := strconv.ParseInt(timeWindowStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "time_window参数无效", Data: nil})
		return
	}

	duration := time.Duration(timeWindow) * time.Second

	// 获取全局队列执行器
	executor := v115open.GetGlobalExecutor()

	// 获取统计数据
	stats := executor.GetStats(duration)

	// 获取限流状态
	throttleStatus := executor.GetThrottleStatus()

	// 构建响应数据
	responseData := gin.H{
		"total_requests":           stats.TotalRequests,
		"qps_count":                stats.QPSCount,
		"qpm_count":                stats.QPMCount,
		"qph_count":                stats.QPHCount,
		"throttled_count":          stats.ThrottledCount,
		"avg_response_time_ms":     stats.AvgResponseTime,
		"last_throttle_time":       stats.LastThrottleTime,
		"throttle_wait_time":       stats.ThrottledWaitTime.String(),
		"throttle_recover_time":    stats.ThrottleRecoverTime,
		"time_window_seconds":      timeWindow,
		"is_throttled":             throttleStatus.IsThrottled,
		"throttled_elapsed_time":   throttleStatus.ElapsedTime.String(),
		"throttled_remaining_time": throttleStatus.RemainingTime.String(),
	}

	c.JSON(http.StatusOK, APIResponse[gin.H]{Code: Success, Message: "获取队列统计数据成功", Data: responseData})
}

// SetQueueRateLimit 设置115 OpenAPI请求队列的速率限制参数
func SetQueueRateLimit(c *gin.Context) {
	var req struct {
		QPS int `json:"qps" binding:"required,min=1,max=1000"`
		QPM int `json:"qpm" binding:"required,min=1,max=100000"`
		QPH int `json:"qph" binding:"required,min=1,max=1000000"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}

	// 设置全局执行器的速率限制配置
	v115open.SetGlobalExecutorConfig(req.QPS, req.QPM, req.QPH)

	helpers.AppLogger.Infof("115 OpenAPI队列速率限制已更新: QPS=%d, QPM=%d, QPH=%d", req.QPS, req.QPM, req.QPH)

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "速率限制配置成功", Data: gin.H{
		"qps": req.QPS,
		"qpm": req.QPM,
		"qph": req.QPH,
	}})
}

// GetRequestStatsByDay 获取指定日期范围内的请求统计（按天分组）
func GetRequestStatsByDay(c *gin.Context) {
	// 获取查询参数
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -7).Format("2006-01-02")) // 默认最近7天
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	// 解析日期
	startDate, err := time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "start_date 参数格式错误，应为 YYYY-MM-DD", Data: nil})
		return
	}
	endDate, err := time.ParseInLocation("2006-01-02", endDateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "end_date 参数格式错误，应为 YYYY-MM-DD", Data: nil})
		return
	}

	// 设置时间范围（开始时间为当天0点，结束时间为当天23:59:59）
	startTime := startDate.Unix()
	endTime := endDate.Add(24*time.Hour - time.Second).Unix()

	// 获取按天分组的统计数据
	dailyStats, err := models.GetDailyRequestStats(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询统计数据失败: " + err.Error(), Data: nil})
		return
	}

	// 获取总请求数和限流请求数
	totalCount, _ := models.GetRequestStatsCount(startTime, endTime)
	throttledCount, _ := models.GetThrottledRequestsCount(startTime, endTime)

	responseData := gin.H{
		"start_date":            startDateStr,
		"end_date":              endDateStr,
		"total_requests":        totalCount,
		"total_throttled":       throttledCount,
		"daily_stats":           dailyStats,
		"query_time_range_days": int(endDate.Sub(startDate).Hours() / 24),
	}

	c.JSON(http.StatusOK, APIResponse[gin.H]{Code: Success, Message: "获取日统计数据成功", Data: responseData})
}

// GetRequestStatsByHour 获取指定日期范围内的请求统计（按小时分组）
func GetRequestStatsByHour(c *gin.Context) {
	// 获取查询参数
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -1).Format("2006-01-02")) // 默认昨天
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))                       // 默认今天

	// 解析日期
	startDate, err := time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "start_date 参数格式错误，应为 YYYY-MM-DD", Data: nil})
		return
	}
	endDate, err := time.ParseInLocation("2006-01-02", endDateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "end_date 参数格式错误，应为 YYYY-MM-DD", Data: nil})
		return
	}

	// 设置时间范围
	startTime := startDate.Unix()
	endTime := endDate.Add(24*time.Hour - time.Second).Unix()

	// 获取按小时分组的统计数据
	hourlyStats, err := models.GetHourlyRequestStats(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询统计数据失败: " + err.Error(), Data: nil})
		return
	}

	// 获取总请求数和限流请求数
	totalCount, _ := models.GetRequestStatsCount(startTime, endTime)
	throttledCount, _ := models.GetThrottledRequestsCount(startTime, endTime)

	responseData := gin.H{
		"start_date":            startDateStr,
		"end_date":              endDateStr,
		"total_requests":        totalCount,
		"total_throttled":       throttledCount,
		"hourly_stats":          hourlyStats,
		"query_time_range_days": int(endDate.Sub(startDate).Hours() / 24),
	}

	c.JSON(http.StatusOK, APIResponse[gin.H]{Code: Success, Message: "获取小时统计数据成功", Data: responseData})
}

// CleanOldRequestStats 清理旧的请求统计数据
func CleanOldRequestStats(c *gin.Context) {
	type cleanReq struct {
		Days int `json:"days" binding:"required,min=1,max=365"`
	}

	var req cleanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}

	err := models.CleanOldRequestStats(req.Days)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "清理统计数据失败: " + err.Error(), Data: nil})
		return
	}

	helpers.AppLogger.Infof("已清理 %d 天前的请求统计数据", req.Days)

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: fmt.Sprintf("已清理 %d 天前的请求统计数据", req.Days), Data: nil})
}
