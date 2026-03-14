package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type LoginRequest struct {
	Username   string `json:"username" form:"username"`
	Password   string `json:"password" form:"password"`
	RememberMe bool   `json:"remember_me" form:"remember_me"`
}

var LoginedUser *models.User = nil

// LoginAction 用户登录
// @Summary 用户登录
// @Description 用户登录并返回JWT Token
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param username body string true "用户名"
// @Param password body string true "密码"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /api/login [post]
func LoginAction(c *gin.Context) {
	user := &models.User{}
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("参数错误：%v", err), Data: nil})
		return
	}
	username := req.Username
	password := req.Password
	if username == "" || password == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "用户名或密码不能为空", Data: nil})
		return
	}

	// 查询用户是否存在
	user, userErr := models.CheckLogin(username, password)
	if userErr != nil || user.ID == 0 {
		c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("用户不存在或者密码错误: %v", userErr), Data: nil})
		return
	}
	claims := &LoginUser{
		ID:       user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(96 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(helpers.GlobalConfig.JwtSecret))
	if err != nil {
		helpers.AppLogger.Errorf("LoginAction: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "登录失败，请重试", Data: nil})
		return
	}
	LoginedUser = user

	// 设置Cookie以支持飞牛App内置浏览器
	maxAge := 3600 // 默认1小时
	if req.RememberMe {
		maxAge = 86400 * 7 // 7天
	}
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    tokenString,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false, // 允许前端读取
		Secure:   false, // 允许HTTP
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)

	res := make(map[string]interface{})
	u := make(map[string]string)
	u["id"] = fmt.Sprintf("%d", user.ID)
	u["username"] = user.Username
	u["email"] = ""
	u["role"] = "admin"
	res["user"] = u
	res["token"] = tokenString
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "登录成功", Data: res})
}

// ChangePassword 修改密码或用户名
// @Summary 修改密码
// @Description 修改当前登录用户的用户名和密码
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param username body string true "新用户名"
// @Param new_password body string true "新密码"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /user/change [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ChangePassword(c *gin.Context) {
	var req struct {
		Username    string `json:"username" form:"username"`
		NewPassword string `json:"new_password" form:"new_password"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("参数错误：%v", err), Data: nil})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "用户名不能为空", Data: nil})
		return
	}
	isChange := false
	isChange2 := false
	var err error
	if req.Username != LoginedUser.Username {
		isChange = true
	}
	isChange2, err = LoginedUser.ChangeUsernameAndPassword(req.Username, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "修改失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "修改成功", Data: isChange || isChange2})
}

// GetUserInfo 获取当前用户信息
// @Summary 获取用户信息
// @Description 获取当前登录用户的ID和用户名
// @Tags 用户管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /user/info [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetUserInfo(c *gin.Context) {
	// 返回当前用户ID和用户名
	respData := make(map[string]string)
	respData["id"] = fmt.Sprintf("%d", LoginedUser.ID)
	respData["username"] = LoginedUser.Username
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取用户信息成功", Data: respData})
}
