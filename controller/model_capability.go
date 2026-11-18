package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type modelCapabilitiesRequest struct {
	Capabilities map[string]json.RawMessage `json:"capabilities"`
	Version      string                     `json:"version"`
}

func GetModelCapabilities(c *gin.Context) {
	listing, err := model.ListExactModelCapabilities()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listing)
}

func PreviewModelCapabilities(c *gin.Context) {
	var request modelCapabilitiesRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	plan, err := model.PlanModelCapabilityReplace(model.DB, request.Capabilities, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, plan)
}

func SaveModelCapabilities(c *gin.Context) {
	var request modelCapabilitiesRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	counts, err := model.ApplyModelCapabilityReplace(request.Capabilities, request.Version)
	if errors.Is(err, model.ErrMetadataSyncConflict) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Model metadata changed. Preview again before saving."})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RefreshPricing()
	common.ApiSuccess(c, counts)
}
