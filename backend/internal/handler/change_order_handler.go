package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/middleware"
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

// Submit 施工方提交变更签证。
func (h *ChangeOrderHandler) Submit(c *gin.Context) {
	var req dto.CreateChangeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	applicantID, _ := c.Get(string(middleware.ContextUserID))
	order, err := h.service.Submit(&req, applicantID.(uint))
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
	page, pageSize := parsePage(c)
	var projectID, nodeID uint
	if projectIDStr := c.Query("project_id"); projectIDStr != "" {
		id, err := parseUintString(projectIDStr)
		if err != nil {
			c.Error(err)
			return
		}
		projectID = id
	}
	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		id, err := parseUintString(nodeIDStr)
		if err != nil {
			c.Error(err)
			return
		}
		nodeID = id
	}
	status := c.Query("status")
	if status != "" && !constants.Contains(constants.ChangeOrderStatuses, status) {
		c.Error(apperrors.NewBadRequest("invalid change order status"))
		return
	}

	// 按项目查询时返回全量列表，便于前端汇总待处理与已批准金额。
	if projectID != 0 {
		orders, err := h.service.ListByProjectID(projectID)
		if err != nil {
			c.Error(err)
			return
		}
		filtered := make([]dto.ChangeOrderDTO, 0, len(orders))
		for i := range orders {
			if nodeID != 0 && orders[i].NodeID != nodeID {
				continue
			}
			if status != "" && orders[i].Status != status {
				continue
			}
			filtered = append(filtered, toChangeOrderDTO(&orders[i]))
		}
		utils.Success(c, filtered)
		return
	}

	orders, total, err := h.service.List(projectID, nodeID, status, page, pageSize)
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
