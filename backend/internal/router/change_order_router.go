package router

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/handler"
)

// registerChangeOrderRoutes 注册施工变更签证路由。
func registerChangeOrderRoutes(group *gin.RouterGroup, h *handler.ChangeOrderHandler, auth gin.HandlerFunc, rbac func(...string) gin.HandlerFunc) {
	group.GET("/change-orders", auth, h.List)
	group.GET("/change-orders/summary", auth, h.Summary)
	group.GET("/change-orders/:id", auth, h.Get)
	group.POST("/change-orders", auth, rbac(constants.RoleAdmin, constants.RoleContractor), h.Create)
	group.PUT("/change-orders/:id/review", auth, rbac(constants.RoleAdmin, constants.RoleProjectManager, constants.RoleOwner), h.Review)
}
