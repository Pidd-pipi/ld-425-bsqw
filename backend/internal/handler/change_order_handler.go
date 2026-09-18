package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/middleware"
	"github.com/home-renovation/platform/internal/repository"
	"github.com/home-renovation/platform/internal/service"
	"github.com/home-renovation/platform/internal/utils"
)

// ChangeOrderHandler 施工变更签证处理器。
type ChangeOrderHandler struct {
	service service.ChangeOrderService
}

// NewChangeOrderHandler 构造变更签证处理器。
func NewChangeOrderHandler(service service.ChangeOrderService) *ChangeOrderHandler {
	return &ChangeOrderHandler{service: service}
}

// Create 施工方提交变更签证。
func (h *ChangeOrderHandler) Create(c *gin.Context) {
	var req dto.CreateChangeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	submitterID, _ := c.Get(string(middleware.ContextUserID))
	order, err := h.service.Create(submitterID.(uint), &req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toChangeOrderDTO(order))
}

// Get 获取变更签证详情。
func (h *ChangeOrderHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	order, err := h.service.GetByID(id)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toChangeOrderDTO(order))
}

// List 获取变更签证列表。
func (h *ChangeOrderHandler) List(c *gin.Context) {
	status := c.Query("status")
	if projectIDStr := c.Query("project_id"); projectIDStr != "" {
		projectID, err := parseUintString(projectIDStr)
		if err != nil {
			c.Error(err)
			return
		}
		orders, err := h.service.ListByProjectID(projectID, status)
		if err != nil {
			c.Error(err)
			return
		}
		utils.Success(c, toChangeOrderDTOList(orders))
		return
	}
	page, pageSize := parsePage(c)
	filter := repository.ChangeOrderFilter{Status: status}
	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		nodeID, err := parseUintString(nodeIDStr)
		if err != nil {
			c.Error(err)
			return
		}
		filter.NodeID = nodeID
	}
	orders, total, err := h.service.List(filter, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, utils.PageResult{List: toChangeOrderDTOList(orders), Total: total, Page: page, PageSize: pageSize})
}

// Review 项目经理或业主审批变更签证。
func (h *ChangeOrderHandler) Review(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.ReviewChangeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	reviewerID, _ := c.Get(string(middleware.ContextUserID))
	order, err := h.service.Review(id, reviewerID.(uint), &req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toChangeOrderDTO(order))
}

// Summary 项目变更汇总：待审批与已批准金额。
func (h *ChangeOrderHandler) Summary(c *gin.Context) {
	projectID, err := parseUintString(c.Query("project_id"))
	if err != nil {
		c.Error(err)
		return
	}
	summary, err := h.service.Summary(projectID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, summary)
}
