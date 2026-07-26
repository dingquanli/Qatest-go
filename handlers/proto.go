package handlers

import (
	"net/http"

	"qatest/models"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// Proto handlers

// GetProtoServices 获取所有服务及方法列表
func GetProtoServices(c *gin.Context) {
	services := services.ProtoLoader.GetServices()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: services})
}

// GetProtoDescribe Proto 总览
func GetProtoDescribe(c *gin.Context) {
	desc := services.ProtoLoader.GetDescribe()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: desc})
}

// GetProtoDir 查询 Proto 目录
func GetProtoDir(c *gin.Context) {
	dir := services.ProtoLoader.GetDir()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"dir": dir}})
}

// SetProtoDir 设置 Proto 目录（仅管理员，防止任意登录用户将 proto 目录指向系统目录）
func SetProtoDir(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "仅管理员可修改 Proto 目录"})
		return
	}

	var req struct {
		Dir string `json:"dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	if err := services.ProtoLoader.SetDir(req.Dir); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"dir": req.Dir}})
}

// DescribeProtoMethod 方法详情
func DescribeProtoMethod(c *gin.Context) {
	var req struct {
		Method string `json:"method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	desc, err := services.ProtoLoader.DescribeMethod(req.Method)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: desc})
}
