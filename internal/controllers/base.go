package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type LoginUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type APIResponseCode int

const (
	Success    APIResponseCode = 200
	BadRequest APIResponseCode = 500
)

type APIResponse[T any] struct {
	Code    APIResponseCode `json:"code"`
	Message string          `json:"message"`
	Data    T               `json:"data"`
}

// JWTAuthMiddleware 基于JWT的认证中间件--验证用户是否登录
func JWTAuthMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 优先检查 API Key（GET 参数 api_key）
		apiKey := c.Query("api_key")
		if apiKey != "" {
			// 验证 API Key
			apiKeyModel, err := models.ValidateAPIKey(apiKey)
			if err == nil && apiKeyModel != nil {
				// API Key 验证成功
				// 获取关联的用户信息
				user, err := models.GetUserById(apiKeyModel.UserID)
				if err == nil && user != nil {
					LoginedUser = user
					// 将用户名保存到上下文
					c.Set("username", user.Username)
					// 异步更新最后使用时间
					go func() {
						apiKeyModel.UpdateLastUsedAt()
					}()
					c.Next()
					return
				}
			}
		}

		// 回退到 JWT Token 验证
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: "Token不存在", Data: nil})
			c.Abort()
			return
		}
		// 按空格分割
		parts := strings.Split(authHeader, ".")
		if len(parts) != 3 {
			c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: "Token格式有误", Data: nil})
			c.Abort()
			return
		}
		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		// helpers.AppLogger.Debugf("tokenString: %s", tokenString)
		loginUser, err := ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("Token无效：%v", err), Data: nil})
			c.Abort()
			return
		}
		// helpers.AppLogger.Debugf("Authenticated user: %s", loginUser.Username)
		LoginedUser, err = models.GetUserById(loginUser.ID)
		if err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("获取用户信息失败：%v", err), Data: nil})
			c.Abort()
			return
		} else {
			// helpers.AppLogger.Debugf("获取用户信息成功: %+v", LoginedUser)
		}
		// 将当前请求的username信息保存到请求的上下文c上
		c.Set("username", loginUser.Username)
		c.Next() // 后续的处理函数可以用过c.Get("username")来获取当前请求的用户信息
	}
}

// ValidateJWT 校验JWT
func ValidateJWT(tokenString string) (*LoginUser, error) {
	token, err := jwt.ParseWithClaims(tokenString, &LoginUser{}, func(token *jwt.Token) (any, error) {
		return []byte(helpers.GlobalConfig.JwtSecret), nil
	})
	// helpers.AppLogger.Debugf("%+v", token)
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("登录凭证校验失败: %v", err)
	}
	claims := token.Claims.(*LoginUser)
	if claims.Username == "" {
		return nil, fmt.Errorf("登录凭证中无法获取用户名")
	}

	return claims, nil
}

func Proxy115(c *gin.Context) {
	// 获取原始url参数
	target := c.Request.URL.Query().Get("url")
	baidupan := c.Request.URL.Query().Get("baidupan")
	if target == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "缺少url参数", Data: nil})
		return
	}
	helpers.AppLogger.Infof("反代网盘下载链接: %s", target)
	// // 只允许反代 cdnfhnfile.115cdn.net 域名
	// if !strings.HasPrefix(target, "https://cdnfhnfile.115cdn.net/") {
	// 	c.JSON(http.StatusForbidden, APIResponse[any]{Code: BadRequest, Message: "只允许反代115CDN链接", Data: nil})
	// 	return
	// }
	// 创建请求
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求创建失败", Data: nil})
		return
	}
	// 复制客户端的 Range、Cookie、Referer 等头部
	for k, v := range c.Request.Header {
		if k == "Range" || k == "Cookie" || k == "Referer" {
			// helpers.AppLogger.Infof("响应头: %s=%s", k, v)
			req.Header[k] = v
		}
	}
	if baidupan != "" {
		req.Header.Set("User-Agent", "pan.baidu.com")
	} else {
		req.Header.Set("User-Agent", v115open.DEFAULTUA)
	}
	// 发起请求
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, APIResponse[any]{Code: BadRequest, Message: "反代请求失败: " + err.Error(), Data: nil})
		return
	}
	defer resp.Body.Close()
	// 复制响应头
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}
	c.Status(resp.StatusCode)
	// 复制响应内容
	_, _ = io.Copy(c.Writer, resp.Body)
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method               //请求方法
		origin := c.Request.Header.Get("Origin") //请求头部
		// var headerKeys []string                  // 声明请求头keys
		// for k := range c.Request.Header {
		// 	headerKeys = append(headerKeys, k)
		// }
		// headerStr := strings.Join(headerKeys, ", ")
		// if headerStr != "" {
		// 	headerStr = fmt.Sprintf("access-control-allow-origin, access-control-allow-headers, %s", headerStr)
		// } else {
		// 	headerStr = "access-control-allow-origin, access-control-allow-headers"
		// }
		if origin != "" {
			origin := c.Request.Header.Get("Origin")
			c.Header("Access-Control-Allow-Origin", origin)                                    // 这是允许访问所有域
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE") //服务器支持的所有跨域请求的方法,为了避免浏览次请求的多次'预检'请求
			//  header的类型
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session,X_Requested_With,Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language,DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma")
			//              允许跨域设置                                                                                                      可以返回其他子段
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,Expires,Last-Modified,Pragma,FooBar") // 跨域关键设置 让浏览器可以解析
			c.Header("Access-Control-Max-Age", "172800")                                                                                                                                                           // 缓存请求信息 单位为秒
			c.Header("Access-Control-Allow-Credentials", "false")                                                                                                                                                  //  跨域请求是否需要带cookie信息 默认设置为true
			c.Set("content-type", "application/json")                                                                                                                                                              // 设置返回格式是json
		}

		// 放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.JSON(http.StatusOK, "Options Request!")
		}
		// 处理请求
		c.Next() //  处理请求
	}
}
