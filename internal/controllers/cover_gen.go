package controllers

import (
	"Q115-STRM/internal/covergen"
	"Q115-STRM/internal/covergen/font"
	"Q115-STRM/internal/helpers"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetCoverGenConfig 获取封面生成配置
// @Summary 获取封面生成配置
// @Description 获取封面生成的全局配置参数
// @Tags 封面生成
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /cover-gen/config [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetCoverGenConfig(c *gin.Context) {
	engine := covergen.GetCoverGenEngine()
	config := engine.GetConfig()

	c.JSON(http.StatusOK, APIResponse[*covergen.CoverGenConfig]{
		Code:    Success,
		Message: "获取配置成功",
		Data:    config,
	})
}

// UpdateCoverGenConfig 更新封面生成配置
// @Summary 更新封面生成配置
// @Description 更新封面生成的全局配置参数
// @Tags 封面生成
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /cover-gen/config [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateCoverGenConfig(c *gin.Context) {
	var config covergen.CoverGenConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	engine := covergen.GetCoverGenEngine()
	if err := engine.UpdateConfig(&config); err != nil {
		helpers.AppLogger.Errorf("更新封面生成配置失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "更新配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: "配置已保存",
	})
}

// GenerateCovers 生成封面并同步到Emby
// @Summary 生成封面并同步到Emby
// @Description 为指定的媒体库生成封面并上传更新到Emby
// @Tags 封面生成
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /cover-gen/generate [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func GenerateCovers(c *gin.Context) {
	var req covergen.CoverGenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	if req.Style != covergen.CoverStyleSingle && req.Style != covergen.CoverStyleGrid {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "不支持的封面样式",
		})
		return
	}

	engine := covergen.GetCoverGenEngine()
	summary, err := engine.GenerateCovers(&req)
	if err != nil {
		helpers.AppLogger.Errorf("生成封面失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: err.Error(),
		})
		return
	}

	msg := "封面生成完成"
	if summary.Failed > 0 {
		msg = "封面生成完成，部分媒体库失败"
	}

	c.JSON(http.StatusOK, APIResponse[*covergen.CoverGenSummary]{
		Code:    Success,
		Message: msg,
		Data:    summary,
	})
}

// PreviewCover 预览封面
// @Summary 预览封面
// @Description 预览指定媒体库的封面效果，不上传
// @Tags 封面生成
// @Accept json
// @Produce image/jpeg
// @Success 200 {object} object
// @Router /cover-gen/preview [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func PreviewCover(c *gin.Context) {
	var req covergen.PreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	if req.Style != covergen.CoverStyleSingle && req.Style != covergen.CoverStyleGrid {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "不支持的封面样式",
		})
		return
	}

	engine := covergen.GetCoverGenEngine()
	coverData, err := engine.PreviewCover(req.LibraryID, req.Style, req.Title)
	if err != nil {
		helpers.AppLogger.Errorf("生成预览封面失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "无法生成预览: " + err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "image/jpeg", coverData)
}

// GetCoverGenStatus 获取封面生成任务状态
// @Summary 获取封面生成任务状态
// @Description 获取当前封面生成任务的运行状态和上次执行结果
// @Tags 封面生成
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /cover-gen/status [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetCoverGenStatus(c *gin.Context) {
	engine := covergen.GetCoverGenEngine()
	status := engine.GetStatus()

	c.JSON(http.StatusOK, APIResponse[*covergen.CoverGenStatus]{
		Code:    Success,
		Message: "",
		Data:    status,
	})
}

// GetFontStatus 获取字体状态
// @Summary 获取字体状态
// @Description 获取中英文字体的可用状态和来源信息
// @Tags 封面生成
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /cover-gen/fonts/status [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetFontStatus(c *gin.Context) {
	fontManager := font.GetFontManager()
	if fontManager == nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "字体管理器未初始化",
		})
		return
	}

	zhAvailable, zhSource, zhPath, zhErr := fontManager.GetFontInfo("zh")
	enAvailable, enSource, enPath, enErr := fontManager.GetFontInfo("en")

	if zhErr != nil {
		helpers.AppLogger.Warnf("获取中文字体状态失败: %v", zhErr)
	}
	if enErr != nil {
		helpers.AppLogger.Warnf("获取英文字体状态失败: %v", enErr)
	}

	status := covergen.FontsStatus{
		ZhFont: covergen.FontStatus{
			Available: zhAvailable,
			Source:    zhSource,
			Path:      zhPath,
		},
		EnFont: covergen.FontStatus{
			Available: enAvailable,
			Source:    enSource,
			Path:      enPath,
		},
	}

	c.JSON(http.StatusOK, APIResponse[*covergen.FontsStatus]{
		Code:    Success,
		Message: "",
		Data:    &status,
	})
}
