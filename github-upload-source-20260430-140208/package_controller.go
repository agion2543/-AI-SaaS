package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type PackageController struct {
	service *service.PackageService
}

func NewPackageController(service *service.PackageService) *PackageController {
	return &PackageController{service: service}
}

func (ctl *PackageController) ListPublished(c *gin.Context) {
	list, err := ctl.service.ListPublished()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *PackageController) ListAll(c *gin.Context) {
	list, err := ctl.service.ListAll()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *PackageController) Save(c *gin.Context) {
	var req dto.SavePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pkg, err := ctl.service.Save(uint(id), req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, pkg)
}
