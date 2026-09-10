package controller

import (
	"net/http"

	"go-web/logger"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type createMaterialRequest struct {
	Key         string `json:"key" binding:"required,min=1,max=64"`
	Title       string `json:"title" binding:"required,min=1,max=128"`
	Kind        string `json:"kind" binding:"required,min=1,max=32"`
	PreviewURL  string `json:"preview_url" binding:"omitempty,max=512"`
	MinVipLevel int    `json:"min_vip_level" binding:"omitempty,min=0,max=2"`
}

func ListMaterials(c *gin.Context) {
	vip := c.GetInt("vip_level")
	list, err := model.ListMaterials(vip)
	if err != nil {
		logger.Log.Error("list materials failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListMaterialFailed)
		return
	}
	resp.OK(c, list)
}

func CreateMaterial(c *gin.Context) {
	var req createMaterialRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	m := &model.Material{
		Key:         req.Key,
		Title:       req.Title,
		Kind:        req.Kind,
		PreviewURL:  req.PreviewURL,
		MinVipLevel: req.MinVipLevel,
		Enabled:     true,
	}
	if err := m.Insert(); err != nil {
		logger.Log.Error("create material failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSaveMaterialFailed)
		return
	}
	resp.Created(c, m)
}
