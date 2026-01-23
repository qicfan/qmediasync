package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/notificationmanager"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/v115open"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Account struct {
	BaseModel
	Name              string      `json:"name"` // 账号备注，仅供用户自己识别账号使用，唯一
	SourceType        SourceType  `json:"source_type"`
	AppId             string      `json:"app_id"`
	Token             string      `json:"token" gorm:"type:string;size:512"`
	RefreshToken      string      `json:"refresh_token" gorm:"type:string;size:512"`
	TokenExpiriesTime int64       `json:"token_expiries_time"`
	UserId            json.Number `json:"user_id"`                                         // 账号对应的用户id，唯一
	Username          string      `json:"username" gorm:"type:string;size:32"`             // 网盘对应的用户名或者openlist的登录用户名
	Password          string      `json:"password" gorm:"type:string;size:256"`            // openlist的用户密码
	BaseUrl           string      `json:"base_url" gorm:"type:string;size:1024"`           // openlist的访问地址http[s]://ip:port
	TokenFailedReason string      `json:"token_failed_reason" gorm:"type:string;size:256"` // 刷新token失败的原因
}

func (account *Account) TableName() string {
	return "account"
}

func (account *Account) GetAppId() string {
	switch account.AppId {
	case "Q115-STRM":
		return helpers.GlobalConfig.Open115AppId
	case "MQ的媒体库":
		return helpers.GlobalConfig.Open115TestAppId
	default:
		return account.AppId
	}
}

// 更新token和refreshToken
func (account *Account) UpdateToken(token string, refreshToken string, expiresTime int64) bool {
	now := time.Now().Unix()
	account.Token = token
	account.RefreshToken = refreshToken
	account.TokenExpiriesTime = now + expiresTime
	account.TokenFailedReason = ""

	updateData := make(map[string]any)
	updateData["token"] = token
	updateData["refresh_token"] = refreshToken
	updateData["token_expiries_time"] = account.TokenExpiriesTime
	updateData["token_failed_reason"] = account.TokenFailedReason
	err := db.Db.Model(account).Where("id = ?", account.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新开放平台登录凭据失败: %v", err)
		return false
	}
	return true
}

// 更新开放平台账号对应的用户信息
func (account *Account) UpdateUser(userId json.Number, username string) bool {
	account.UserId = userId
	account.Username = username
	updateData := make(map[string]any)
	updateData["user_id"] = userId
	updateData["username"] = username
	err := db.Db.Model(account).Where("id = ?", account.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新开放平台账号用户信息失败: %v", err)
		return false
	}
	// helpers.AppLogger.Debugf("更新开放平台账号用户信息成功: %v", account)
	return true
}

// 如果是normal模式，创建一个新的客户端，不启用限速器
func (account *Account) Get115Client(normal bool) *v115open.OpenClient {
	qps := SettingsGlobal.FileDetailThreads
	if normal {
		qps = 0
	}
	appId := account.GetAppId()
	return v115open.GetClient(qps, account.ID, appId, account.Token, account.RefreshToken)
}

func (account *Account) GetOpenListClient() *openlist.Client {
	return openlist.NewClient(account.ID, account.BaseUrl, account.Username, account.Password, account.Token)
}

func (account *Account) Delete() error {
	// 检查是否有关联的同步目录没有删除
	syncPaths := GetAllSyncPathByAccountId(account.ID)
	if len(syncPaths) > 0 {
		helpers.AppLogger.Errorf("开放平台账号 %v 有关联的同步目录，不能删除", account.ID)
		return fmt.Errorf("开放平台账号 %v 有关联的同步目录，不能删除", account.ID)
	}

	err := db.Db.Delete(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("删除开放平台账号失败: %v", err)
		return err
	}
	return nil
}

func (account *Account) ClearToken(reason string) {
	account.Token = ""
	account.RefreshToken = ""
	account.TokenExpiriesTime = 0
	account.TokenFailedReason = reason
	// 保存到数据库
	err := db.Db.Save(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("清空开放平台访问凭证失败: %v", err)
		return
	}
}

func (account *Account) UpdateOpenList(baseUrl string, username string, password string) error {
	oldUsername := account.Username
	oldPassword := account.Password
	oldBaseUrl := account.BaseUrl
	account.BaseUrl = baseUrl
	account.Username = username
	account.Password = password

	if oldUsername != account.Username || oldPassword != account.Password {
		// 重新获取token
		client := account.GetOpenListClient()
		tokenData, err := client.GetToken()
		if err != nil {
			helpers.AppLogger.Errorf("更新openlist账号token失败: %v", err)
			// 还原账号信息
			account.BaseUrl = oldBaseUrl
			account.Username = oldUsername
			account.Password = oldPassword
			return err
		}
		account.Token = tokenData.Token
	}
	// 保存到数据库
	err := db.Db.Save(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新openlist账号失败: %v", err)
		return err
	}
	return nil
}

// 使用name创建一个临时账号，用户后续授权绑定
// name: 账号备注
func CreateAccountByName(name string, srouceType SourceType, appId string) (*Account, error) {
	account := &Account{}
	account.Name = name
	account.SourceType = srouceType
	account.AppId = appId
	account.Token = ""
	account.RefreshToken = ""
	account.TokenExpiriesTime = 0
	account.UserId = ""
	account.Username = ""

	// 插入数据库，如果插入失败则报错
	err := db.Db.Create(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("创建开放平台账号失败: %v", err)
		return nil, err
	}
	return account, nil
}

// 创建openlist账号
// baseUrl: openlist的访问地址
// username: openlist的登录用户名
// password: openlist的登录密码
func CreateOpenListAccount(baseUrl string, username string, password string) (*Account, error) {
	account := &Account{}
	account.Name = username
	account.SourceType = SourceTypeOpenList
	account.AppId = ""
	account.BaseUrl = baseUrl
	account.Username = username
	account.Password = password
	// 验证账号是否正确，获取一次token
	client := account.GetOpenListClient()
	tokenData, clientErr := client.GetToken()
	if clientErr != nil {
		helpers.AppLogger.Errorf("验证openlist账号失败: %v", clientErr)
		return nil, clientErr
	} else {
		helpers.AppLogger.Infof("获取openlist账号token成功")
	}
	account.Token = tokenData.Token
	// 插入数据库，如果插入失败则报错
	err := db.Db.Create(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("创建openlist账号失败: %v", err)
		return nil, err
	}
	return account, nil
}

// 创建115账号，如果userId已经存在，则更新
// token: 115账号的token
// refreshToken: 115账号的refreshToken
// userId: 115账号对应的用户id
// username: 115账号对应的用户名
// expiresTime: token的过期时间
func CreateAccountFull(sourceType SourceType, AppId string, name string, token string, refreshToken string, userId json.Number, username string, expiresTime int64) *Account {
	// 先检查userId是否已经存在
	account, err := GetAccountByUserId(userId)
	updateOrCreate := "create"
	if err == nil {
		// 说明userId已经存在
		helpers.AppLogger.Errorf("开放平台账号对应的用户id已经存在: %v", userId)
		updateOrCreate = "update"
	} else {
		account = &Account{}
	}
	now := time.Now().Unix()
	account.SourceType = sourceType
	account.AppId = AppId
	account.Name = name
	account.Token = token
	account.RefreshToken = refreshToken
	account.TokenExpiriesTime = now + expiresTime
	account.UserId = userId
	account.Username = username
	if updateOrCreate == "update" {
		err := db.Db.Save(account).Error
		if err != nil {
			helpers.AppLogger.Errorf("保存开放平台账号失败: %v", err)
			return nil
		}
		return account
	} else {
		err := db.Db.Create(account).Error
		if err != nil {
			helpers.AppLogger.Errorf("创建开放平台账号失败: %v", err)
			return nil
		}
		return account
	}
}

// 通过userid查询开放平台账号
func GetAccountByUserId(userId json.Number) (*Account, error) {
	account := &Account{}
	err := db.Db.Where("user_id = ?", userId).First(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询开放平台账号失败: %v", err)
		return nil, err
	}
	return account, nil
}

// 通过ID查询开放平台账号
func GetAccountById(id uint) (*Account, error) {
	account := &Account{}
	err := db.Db.Where("id = ?", id).First(account).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询开放平台账号失败: %v", err)
		return nil, err
	}
	return account, nil
}

// 通过sourceType查询account列表
func GetAccountBySourceType(sourceType SourceType) ([]*Account, error) {
	accounts := []*Account{}
	err := db.Db.Where("source_type = ?", sourceType).Find(&accounts).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询开放平台账号失败: %v", err)
		return nil, err
	}
	return accounts, nil
}

// 查询account列表，全部返回
func GetAllAccount() ([]Account, error) {
	var accounts []Account
	err := db.Db.Order("id desc").Find(&accounts).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询开放平台账号失败: %v", err)
		return nil, err
	}
	return accounts, nil
}

// 根据fileId获取文件夹的路径
func GetPathByPathFileId(account *Account, fileId string) string {
	client := account.Get115Client(true)
	ctx := context.Background()
	detail, err := client.GetFsDetailByCid(ctx, fileId)
	if err != nil {
		helpers.AppLogger.Errorf("查询文件详情失败: %v", err)
		return ""
	}
	// 生成完整路径
	baseDir := detail.GetFullPath()
	return baseDir + "/" + detail.FileName
}

// 处理115访问凭证失效事件（异步版本）
func HandleV115TokenInvalid(event helpers.Event) helpers.EventResult {
	eventData := event.Data.(map[string]interface{})
	helpers.AppLogger.Infof("收到V115访问凭证失效事件，开始处理，账号ID：%d", eventData["account_id"].(uint))
	account, err := GetAccountById(eventData["account_id"].(uint))
	if err != nil {
		helpers.AppLogger.Errorf("查询开放平台账号失败: %v", err)
		return helpers.EventResult{
			Success: false,
			Error:   err,
			Data:    nil,
		}
	}
	account.ClearToken(eventData["reason"].(string))
	ctx := context.Background()
	notif := &Notification{
		Type:      SystemAlert,
		Title:     "🔐 115开放平台访问凭证已失效",
		Content:   fmt.Sprintf("账号ID：%d\n用户名：%s\n请重新授权\n⏰ 时间: %s", int(account.ID), account.Username, time.Now().Format("2006-01-02 15:04:05")),
		Timestamp: time.Now(),
		Priority:  HighPriority,
	}
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
			helpers.AppLogger.Errorf("发送访问凭证失效通知失败: %v", err)
		}
	}
	return helpers.EventResult{
		Success: true,
		Error:   nil,
		Data:    nil,
	}
}

// 处理OpenList访问凭证保存事件（同步版本）
func HandleOpenListTokenSaveSync(event helpers.Event) helpers.EventResult {
	helpers.AppLogger.Warnf("收到OpenList访问凭证保存同步事件，开始处理")

	eventData := event.Data.(map[string]any)
	account, err := GetAccountById(eventData["account_id"].(uint))
	if err != nil {
		helpers.AppLogger.Errorf("查询OpenList账号失败: %v", err)
		return helpers.EventResult{
			Success: false,
			Error:   err,
			Data:    nil,
		}
	}
	// expiresTime = now+ 48小时
	expiresTime := int64(48 * 60 * 60)
	suc := account.UpdateToken(eventData["token"].(string), "", expiresTime)

	if suc {
		helpers.AppLogger.Infof("OpenList访问凭证保存成功")
		return helpers.EventResult{
			Success: true,
			Error:   nil,
			Data:    nil,
		}
	} else {
		helpers.AppLogger.Warn("OpenList访问凭证保存失败")
		return helpers.EventResult{
			Success: false,
			Error:   fmt.Errorf("OpenList访问凭证保存失败"),
			Data:    nil,
		}
	}
}
