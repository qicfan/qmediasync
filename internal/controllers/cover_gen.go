package controllers

import (
	"Q115-STRM/internal/covergen"
	"Q115-STRM/internal/covergen/font"
	"Q115-STRM/internal/helpers"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	Success = 0
	BadRequest = 1
	InternalServerError = 2
)

type APIResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func GetCoverGenConfig(c *gin.Context) {
	engine := covergen.GetCoverGenEngine()
	config := engine.GetConfig()
	
	c.JSON(http.StatusOK, APIResponse[*covergen.CoverGenConfig]{
		Code:    Success,
		Message: "获取配置成功",
		Data:    config,
	})
}

func UpdateCoverGenConfig(c *gin.Context) {
	var config covergen.CoverGenConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "请求参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}
	
	engine := covergen.GetCoverGenEngine()
	if err := engine.UpdateConfig(&config); err != nil {
		helpers.AppLogger.Errorf("更新封面生成配置失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    InternalServerError,
			Message: "更新配置失败: " + err.Error(),
			Data:    nil,
		})
		return
	}
	
	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: "配置已保存",
		Data:    nil,
	})
}

func GenerateCovers(c *gin.Context) {
	var req covergen.CoverGenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "请求参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}
	
	if req.Style != covergen.CoverStyleSingle && req.Style != covergen.CoverStyleGrid {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "不支持的封面样式",
			Data:    nil,
		})
		return
	}
	
	engine := covergen.GetCoverGenEngine()
	summary, err := engine.GenerateCovers(&req)
	if err != nil {
		helpers.AppLogger.Errorf("生成封面失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    InternalServerError,
			Message: err.Error(),
			Data:    nil,
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

func PreviewCover(c *gin.Context) {
	var req covergen.PreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "请求参数错误: " + err.Error(),
			Data:    nil,
		})
		return
	}
	
	if req.Style != covergen.CoverStyleSingle && req.Style != covergen.CoverStyleGrid {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    BadRequest,
			Message: "不支持的封面样式",
			Data:    nil,
		})
		return
	}
	
	engine := covergen.GetCoverGenEngine()
	coverData, err := engine.PreviewCover(req.LibraryID, req.Style, req.Title)
	if err != nil {
		helpers.AppLogger.Errorf("生成预览封面失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    InternalServerError,
			Message: "无法生成预览: " + err.Error(),
			Data:    nil,
		})
		return
	}
	
	c.Data(http.StatusOK, "image/jpeg", coverData)
}

func GetCoverGenStatus(c *gin.Context) {
	engine := covergen.GetCoverGenEngine()
	status := engine.GetStatus()
	
	c.JSON(http.StatusOK, APIResponse[*covergen.CoverGenStatus]{
		Code:    Success,
		Message: "获取状态成功",
		Data:    status,
	})
}

func GetFontStatus(c *gin.Context) {
	fontManager := font.GetFontManager()
	if fontManager == nil {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    InternalServerError,
			Message: "字体管理器未初始化",
			Data:    nil,
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
		Message: "获取字体状态成功",
		Data:    &status,
	})
}
