package handler

import (
	"net/http"
	"strconv"
	"web_backend/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

func (h *Handler) GetFeed(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.Redirect(http.StatusFound, "/feed/1")
		return
	}

	var factor repository.RiskFactor

	if ctx.Query("next") == "true" {
		factor, err = h.Repository.GetNextRiskFactor(id)
	} else if ctx.Query("prev") == "true" {
		factor, err = h.Repository.GetPrevRiskFactor(id)
	} else {
		factor, err = h.Repository.GetRiskFactor(id)
	}

	if err != nil {
		ctx.Redirect(http.StatusFound, "/feed/1")
		return
	}

	if factor.Status == "deleted" {
		ctx.Redirect(http.StatusFound, "/feed/1")
		return
	}

	ctx.HTML(http.StatusOK, "feed.html", gin.H{
		"factor": factor,
	})
}

func (h *Handler) GetAdd(ctx *gin.Context) {
	factor := repository.RiskFactor{}
	ctx.HTML(http.StatusOK, "add.html", gin.H{
		"factor": factor,
	})
}

func (h *Handler) GetGrid(ctx *gin.Context) {
	var factors []repository.RiskFactor
	var err error

	maxAgeStr := ctx.Query("maxAge")

	if maxAgeStr != "" {
		maxAge, _ := strconv.Atoi(maxAgeStr)
		factors, err = h.Repository.GetRiskFactorByMaxAge(maxAge)
	} else {
		factors, err = h.Repository.GetRiskFactors()
	}

	if err != nil {
		logrus.Error(err)
	}

	var visibleFactors []repository.RiskFactor
	for _, f := range factors {
		if f.Status != "deleted" {
			visibleFactors = append(visibleFactors, f)
		}
	}

	ctx.HTML(http.StatusOK, "grid.html", gin.H{
		"factors": visibleFactors,
		"maxAge":  maxAgeStr,
	})
}
