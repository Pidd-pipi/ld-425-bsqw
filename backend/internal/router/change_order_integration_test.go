package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/config"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/handler"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"github.com/home-renovation/platform/internal/service"
	"gorm.io/gorm"
)

// newTestEngine 使用内存 SQLite 装配完整应用，验证变更签证闭环的 HTTP 行为。
func newTestEngine(t *testing.T) (*httptest.Server, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.RenovationProject{},
		&model.DesignPhase{},
		&model.MaterialItem{},
		&model.BudgetItem{},
		&model.ConstructionNode{},
		&model.ChangeOrder{},
		&model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	log := slog.Default()
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "integration-test-secret"}}

	projectRepo := repository.NewProjectRepository(db)
	designRepo := repository.NewDesignRepository(db)
	materialRepo := repository.NewMaterialRepository(db)
	budgetRepo := repository.NewBudgetRepository(db)
	constructionRepo := repository.NewConstructionRepository(db)
	changeOrderRepo := repository.NewChangeOrderRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	txManager := repository.NewTransactionManager(db)

	userSvc := service.NewUserService(userRepo, cfg.JWT, log)
	auditSvc := service.NewAuditService(auditRepo, log)
	projectSvc := service.NewProjectService(projectRepo, log)
	designSvc := service.NewDesignService(designRepo, log)
	materialSvc := service.NewMaterialService(materialRepo, log)
	budgetSvc := service.NewBudgetService(budgetRepo, log)
	constructionSvc := service.NewConstructionService(constructionRepo, log)
	changeOrderSvc := service.NewChangeOrderService(changeOrderRepo, projectRepo, budgetRepo, constructionRepo, txManager, log)

	if err := userSvc.SeedIfEmpty(); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	engine := New(Deps{
		Config:        cfg,
		Logger:        log,
		UserSvc:       userSvc,
		AuditSvc:      auditSvc,
		AuditRepo:     auditRepo,
		ProjectH:      handler.NewProjectHandler(projectSvc),
		DesignH:       handler.NewDesignHandler(designSvc),
		MaterialH:     handler.NewMaterialHandler(materialSvc),
		BudgetH:       handler.NewBudgetHandler(budgetSvc),
		ConstructionH: handler.NewConstructionHandler(constructionSvc),
		ChangeOrderH:  handler.NewChangeOrderHandler(changeOrderSvc),
		AuditH:        handler.NewAuditHandler(auditSvc),
		UploadH:       handler.NewUploadHandler(),
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	return server, db
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func loginToken(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil || data.Token == "" {
		t.Fatalf("login %s failed: code=%d msg=%s", username, env.Code, env.Message)
	}
	return data.Token
}

func doRequest(t *testing.T, method, url, token string, body interface{}) (int, apiEnvelope) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, env
}

func TestChangeOrderClosedLoop(t *testing.T) {
	server, db := newTestEngine(t)
	base := server.URL

	contractorToken := loginToken(t, base, "contractor", "Contractor123")
	pmToken := loginToken(t, base, "pm", "Manager123456")
	ownerToken := loginToken(t, base, "owner", "Owner123456")

	// 准备项目（合同额 100000）、预算项（已用 40000）与施工节点。
	project := model.RenovationProject{
		Name: "变更测试项目", HouseType: "三室", Area: 100, DecorStyle: "Modern",
		OwnerID: 4, Status: "InProgress", ContractAmount: 100000,
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	budget := model.BudgetItem{ProjectID: project.ID, Category: "Labor", BudgetAmount: 50000, ActualAmount: 40000, Variance: -10000}
	if err := db.Create(&budget).Error; err != nil {
		t.Fatalf("seed budget: %v", err)
	}
	node := model.ConstructionNode{ProjectID: project.ID, Name: "水电", Status: "InProgress", AcceptanceStatus: "Pending"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	node2 := model.ConstructionNode{ProjectID: project.ID, Name: "木工", Status: "Pending", AcceptanceStatus: "Pending"}
	if err := db.Create(&node2).Error; err != nil {
		t.Fatalf("seed node2: %v", err)
	}

	// 1. 施工方提交变更签证。
	createBody := map[string]interface{}{
		"project_id": project.ID, "node_id": node.ID,
		"title": "客厅增加插座回路", "amount": 10000, "schedule_impact_days": 3,
	}
	status, env := doRequest(t, http.MethodPost, base+"/api/v1/change-orders", contractorToken, createBody)
	if status != http.StatusOK {
		t.Fatalf("create change order: status=%d msg=%s", status, env.Message)
	}
	var order dto.ChangeOrderDTO
	if err := json.Unmarshal(env.Data, &order); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	if order.Status != "Pending" || order.ID == 0 {
		t.Fatalf("expected pending order, got %+v", order)
	}

	// 2. 同一节点只能有一个待审批变更。
	status, _ = doRequest(t, http.MethodPost, base+"/api/v1/change-orders", contractorToken, createBody)
	if status != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate pending, got %d", status)
	}

	// 3. 待审批不计入已用预算。
	status, env = doRequest(t, http.MethodGet, fmt.Sprintf("%s/api/v1/change-orders/summary?project_id=%d", base, project.ID), pmToken, nil)
	if status != http.StatusOK {
		t.Fatalf("summary: status=%d", status)
	}
	var summary dto.ChangeOrderSummaryDTO
	if err := json.Unmarshal(env.Data, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.UsedBudget != 40000 || summary.PendingAmount != 10000 || summary.ApprovedAmount != 0 {
		t.Fatalf("unexpected summary before review: %+v", summary)
	}

	// 4. 施工方无权审批。
	status, _ = doRequest(t, http.MethodPut, fmt.Sprintf("%s/api/v1/change-orders/%d/review", base, order.ID), contractorToken, map[string]interface{}{"approved": true})
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for contractor review, got %d", status)
	}

	// 5. 项目经理批准：变更单、预算项、项目汇总一并生效。
	status, env = doRequest(t, http.MethodPut, fmt.Sprintf("%s/api/v1/change-orders/%d/review", base, order.ID), pmToken, map[string]interface{}{"approved": true, "note": "同意增项"})
	if status != http.StatusOK {
		t.Fatalf("approve: status=%d msg=%s", status, env.Message)
	}
	var approved dto.ChangeOrderDTO
	if err := json.Unmarshal(env.Data, &approved); err != nil {
		t.Fatalf("decode approved: %v", err)
	}
	if approved.Status != "Approved" {
		t.Fatalf("expected Approved, got %q", approved.Status)
	}
	var usedSum float64
	if err := db.Model(&model.BudgetItem{}).Where("project_id = ?", project.ID).Select("COALESCE(SUM(actual_amount),0)").Scan(&usedSum).Error; err != nil {
		t.Fatalf("sum actual: %v", err)
	}
	if usedSum != 50000 {
		t.Fatalf("expected used budget 50000 after approval, got %v", usedSum)
	}
	var updated model.RenovationProject
	if err := db.First(&updated, project.ID).Error; err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if updated.ApprovedChangeAmount != 10000 {
		t.Fatalf("expected project change summary 10000, got %v", updated.ApprovedChangeAmount)
	}

	// 6. 重复审批只允许首个成功。
	status, _ = doRequest(t, http.MethodPut, fmt.Sprintf("%s/api/v1/change-orders/%d/review", base, order.ID), ownerToken, map[string]interface{}{"approved": false})
	if status != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate review, got %d", status)
	}

	// 7. 超出合同额与已用预算差额的变更整单拒绝。
	bigBody := map[string]interface{}{"project_id": project.ID, "node_id": node2.ID, "title": "全屋定制升级", "amount": 60000}
	status, env = doRequest(t, http.MethodPost, base+"/api/v1/change-orders", contractorToken, bigBody)
	if status != http.StatusOK {
		t.Fatalf("create big order: status=%d msg=%s", status, env.Message)
	}
	var bigOrder dto.ChangeOrderDTO
	if err := json.Unmarshal(env.Data, &bigOrder); err != nil {
		t.Fatalf("decode big order: %v", err)
	}
	// 已用 50000 + 60000 > 合同额 100000。
	status, _ = doRequest(t, http.MethodPut, fmt.Sprintf("%s/api/v1/change-orders/%d/review", base, bigOrder.ID), pmToken, map[string]interface{}{"approved": true})
	if status != http.StatusConflict {
		t.Fatalf("expected 409 for over-budget approval, got %d", status)
	}
	var stillPending model.ChangeOrder
	if err := db.First(&stillPending, bigOrder.ID).Error; err != nil {
		t.Fatalf("reload big order: %v", err)
	}
	if stillPending.Status != "Pending" {
		t.Fatalf("expected big order to stay pending, got %q", stillPending.Status)
	}
	if err := db.Model(&model.BudgetItem{}).Where("project_id = ?", project.ID).Select("COALESCE(SUM(actual_amount),0)").Scan(&usedSum).Error; err != nil {
		t.Fatalf("sum actual: %v", err)
	}
	if usedSum != 50000 {
		t.Fatalf("expected used budget unchanged 50000, got %v", usedSum)
	}

	// 8. 业主驳回，预算不变。
	status, env = doRequest(t, http.MethodPut, fmt.Sprintf("%s/api/v1/change-orders/%d/review", base, bigOrder.ID), ownerToken, map[string]interface{}{"approved": false, "note": "超出预算"})
	if status != http.StatusOK {
		t.Fatalf("reject: status=%d msg=%s", status, env.Message)
	}
	var rejected dto.ChangeOrderDTO
	if err := json.Unmarshal(env.Data, &rejected); err != nil {
		t.Fatalf("decode rejected: %v", err)
	}
	if rejected.Status != "Rejected" {
		t.Fatalf("expected Rejected, got %q", rejected.Status)
	}
	if err := db.Model(&model.BudgetItem{}).Where("project_id = ?", project.ID).Select("COALESCE(SUM(actual_amount),0)").Scan(&usedSum).Error; err != nil {
		t.Fatalf("sum actual: %v", err)
	}
	if usedSum != 50000 {
		t.Fatalf("expected used budget unchanged after reject, got %v", usedSum)
	}

	// 9. 驳回后同节点可再次提交；汇总反映已批准金额。
	status, env = doRequest(t, http.MethodPost, base+"/api/v1/change-orders", contractorToken, bigBody)
	if status != http.StatusOK {
		t.Fatalf("resubmit after reject: status=%d msg=%s", status, env.Message)
	}
	status, env = doRequest(t, http.MethodGet, fmt.Sprintf("%s/api/v1/change-orders/summary?project_id=%d", base, project.ID), pmToken, nil)
	if status != http.StatusOK {
		t.Fatalf("summary after loop: status=%d", status)
	}
	if err := json.Unmarshal(env.Data, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.ApprovedAmount != 10000 || summary.UsedBudget != 50000 || summary.RemainingAmount != 50000 || summary.PendingAmount != 60000 {
		t.Fatalf("unexpected final summary: %+v", summary)
	}

	// 10. 列表接口按项目返回全部变更单。
	status, env = doRequest(t, http.MethodGet, fmt.Sprintf("%s/api/v1/change-orders?project_id=%d", base, project.ID), contractorToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list: status=%d", status)
	}
	var orders []dto.ChangeOrderDTO
	if err := json.Unmarshal(env.Data, &orders); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}
}
