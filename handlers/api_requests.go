package handlers

import (
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// --- API 请求管理 ---

func GetAPIRequests(c *gin.Context) {
	reqs, err := repository.ListAPIRequests()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: reqs})
}

func GetAPIRequest(c *gin.Context) {
	r, err := repository.GetAPIRequest(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "请求不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: r})
}

func CreateAPIRequest(c *gin.Context) {
	var r models.APIRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	r.ID = generateID("ar")
	r.CreatedAt = models.NowStr()
	r.UpdatedAt = r.CreatedAt
	if err := repository.CreateAPIRequest(r); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: r})
}

func UpdateAPIRequest(c *gin.Context) {
	id := c.Param("id")
	var r models.APIRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	r.UpdatedAt = models.NowStr()
	if err := repository.UpdateAPIRequest(id, r); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	r.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: r})
}

func DeleteAPIRequest(c *gin.Context) {
	if err := repository.DeleteAPIRequest(c.Param("id")); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- API 文件夹（SQL 收敛于 module_crud.go） ---

func GetAPIFolders(c *gin.Context)   { listModules(c, tblAPIFolders) }
func CreateAPIFolder(c *gin.Context) { createModule(c, tblAPIFolders, "af") }
func UpdateAPIFolder(c *gin.Context) { updateModule(c, tblAPIFolders) }
func DeleteAPIFolder(c *gin.Context) { deleteModule(c, tblAPIFolders) }

// --- API 请求历史 ---

func GetAPIHistory(c *gin.Context) {
	hist, err := repository.ListAPIHistory()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: hist})
}

func CreateAPIHistory(c *gin.Context) {
	var h models.APIHistory
	if err := c.ShouldBindJSON(&h); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	h.ID = generateID("ah")
	h.CreatedAt = models.NowStr()
	if err := repository.CreateAPIHistory(h); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: h})
}

func ClearAPIHistory(c *gin.Context) {
	if err := repository.ClearAPIHistory(); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
