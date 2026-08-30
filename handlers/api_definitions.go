package handlers

import (
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// --- 接口定义 ---

func GetAPIDefinitions(c *gin.Context) {
	defs, err := repository.ListAPIDefinitions()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: defs})
}

func GetAPIDefinition(c *gin.Context) {
	d, err := repository.GetAPIDefinition(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "接口定义不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: d})
}

func CreateAPIDefinition(c *gin.Context) {
	var d models.APIDefinition
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	d.ID = generateID("ad")
	d.CreatedAt = models.NowStr()
	d.UpdatedAt = d.CreatedAt
	if err := repository.CreateAPIDefinition(d); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: d})
}

func UpdateAPIDefinition(c *gin.Context) {
	id := c.Param("id")
	var d models.APIDefinition
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	d.UpdatedAt = models.NowStr()
	if err := repository.UpdateAPIDefinition(id, d); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	d.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: d})
}

func DeleteAPIDefinition(c *gin.Context) {
	if err := repository.DeleteAPIDefinition(c.Param("id")); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- 接口定义模块（SQL 收敛于 module_crud.go） ---

func GetAPIDefModules(c *gin.Context)   { listModules(c, tblAPIDefModules) }
func CreateAPIDefModule(c *gin.Context) { createModule(c, tblAPIDefModules, "adm") }
func UpdateAPIDefModule(c *gin.Context) { updateModule(c, tblAPIDefModules) }
func DeleteAPIDefModule(c *gin.Context) { deleteModule(c, tblAPIDefModules) }
