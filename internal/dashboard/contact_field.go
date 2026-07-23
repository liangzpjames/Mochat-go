package dashboard

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ContactFieldFilter struct {
	Status  int
	Page    int
	PerPage int
}

type ContactField struct {
	ID      int
	Name    string
	Label   string
	Type    int
	Options any
	Status  int
	Order   int
	IsSys   int
}

type ContactFieldPage struct {
	Items     []ContactField
	Total     int
	TotalPage int
	PerPage   int
}

type ContactFieldPivot struct {
	ID             int
	ContactFieldID int
	Value          string
}

type ContactFieldPivotCreate struct {
	ContactID      int
	ContactFieldID int
	Value          string
}

type ContactEmployeeTrackCreate struct {
	EmployeeID int
	ContactID  int
	Content    string
	CorpID     int
	Event      int
}

type ContactFieldWriteValues struct {
	Name    string
	Label   string
	Type    int
	Options string
	Order   int
	Status  int
}

type ContactFieldBatchUpdate struct {
	ID     int
	Values ContactFieldWriteValues
}

type SidebarEmployee struct {
	ID        int
	CorpID    int
	LogUserID int
}

type ContactFieldStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	ContactFieldPage(ctx context.Context, filter ContactFieldFilter) (ContactFieldPage, error)
	ContactFieldByID(ctx context.Context, fieldID int) (ContactField, bool, error)
	ContactFieldsByStatusOrder(ctx context.Context, status int) ([]ContactField, error)
	ContactFieldPivotsByContactID(ctx context.Context, contactID int, fieldIDs []int) (map[int]ContactFieldPivot, error)
	ContactFieldPivotByID(ctx context.Context, pivotID int) (ContactFieldPivot, bool, error)
	UpdateContactFieldPivotValue(ctx context.Context, pivotID int, value string) (bool, error)
	CreateContactFieldPivots(ctx context.Context, pivots []ContactFieldPivotCreate) error
	CreateContactEmployeeTrack(ctx context.Context, track ContactEmployeeTrackCreate) error
	ContactFieldLabelExists(ctx context.Context, label string, excludeFieldID int) (bool, error)
	CreateContactField(ctx context.Context, values ContactFieldWriteValues) (int, error)
	UpdateContactField(ctx context.Context, fieldID int, values ContactFieldWriteValues) (bool, error)
	UpdateContactFieldStatus(ctx context.Context, fieldID int, status int) (bool, error)
	DeleteContactField(ctx context.Context, fieldID int) (bool, error)
	BatchUpdateContactFields(ctx context.Context, updates []ContactFieldBatchUpdate, destroyIDs []int) error
}

type ContactFieldHandler struct {
	store      ContactFieldStore
	cache      LoginCache
	resolver   UserIDResolver
	sidebar    UserIDResolver
	authorizer CorpAdminAuthorizer
	apiBaseURL string
}

func NewContactFieldHandler(store ContactFieldStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string) *ContactFieldHandler {
	return &ContactFieldHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, apiBaseURL: strings.TrimRight(apiBaseURL, "/")}
}

func (h *ContactFieldHandler) WithSidebarEmployeeResolver(resolver UserIDResolver) *ContactFieldHandler {
	h.sidebar = resolver
	return h
}

func (h *ContactFieldHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/index#get", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}

	filter := ContactFieldFilter{
		Status:  positiveQueryInt(r, "status", 2),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 10),
	}
	page, err := h.store.ContactFieldPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, field := range page.Items {
		list = append(list, contactFieldPayload(field))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *ContactFieldHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/show#get", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}

	fieldID, err := positiveQueryIntRequired(r, "id")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
		return
	}
	field, found, err := h.store.ContactFieldByID(r.Context(), fieldID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "无此条信息", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", contactFieldPayload(field))
}

func (h *ContactFieldHandler) Portrait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}

	fields, err := h.store.ContactFieldsByStatusOrder(r.Context(), 1)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(fields) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}

	list := []any{map[string]any{"fieldId": 0, "name": "全部"}}
	for _, field := range fields {
		if field.Type == 11 {
			continue
		}
		list = append(list, map[string]any{
			"fieldId":  field.ID,
			"name":     field.Label,
			"type":     field.Type,
			"typeText": contactFieldTypeText(field.Type),
			"options":  field.Options,
		})
	}
	list = append(list, []any{})
	writeEnvelope(w, http.StatusOK, 200, "", list)
}

func (h *ContactFieldHandler) FieldPivotIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactFieldPivot/index#get", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}

	h.writeFieldPivotIndex(w, r)
}

func (h *ContactFieldHandler) SidebarFieldPivotIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolveSidebarAccess(w, r); !ok {
		return
	}

	h.writeFieldPivotIndex(w, r)
}

func (h *ContactFieldHandler) FieldPivotUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	h.writeFieldPivotUpdate(w, r, params, loginInfo.WorkEmployeeID, corpID)
}

func (h *ContactFieldHandler) SidebarFieldPivotUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	h.writeFieldPivotUpdate(w, r, params, employee.ID, employee.CorpID)
}

func (h *ContactFieldHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/store#post", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	values, ok := h.parseContactFieldValues(w, params, ContactField{}, false)
	if !ok {
		return
	}
	exists, err := h.store.ContactFieldLabelExists(r.Context(), values.Label, 0)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, fmt.Sprintf("属性[%s]已经存在", values.Label), nil)
		return
	}
	if _, err := h.store.CreateContactField(r.Context(), values); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "添加失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactFieldHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/update#put", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	fieldID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || fieldID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
		return
	}
	field, found, err := h.store.ContactFieldByID(r.Context(), fieldID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
		return
	}
	values, ok := h.parseContactFieldValues(w, params, field, true)
	if !ok {
		return
	}
	if field.IsSys == 0 {
		exists, err := h.store.ContactFieldLabelExists(r.Context(), values.Label, fieldID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if exists {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, fmt.Sprintf("属性[%s]已经存在", values.Label), nil)
			return
		}
	}
	updated, err := h.store.UpdateContactField(r.Context(), fieldID, values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "修改失败", nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactFieldHandler) StatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/statusUpdate#put", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	fieldID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || fieldID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
		return
	}
	status, okInt, err := intParam(params, "status")
	if err != nil || !okInt || (status != 0 && status != 1) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态必须在 0, 1中选择一个", nil)
		return
	}
	if _, found, err := h.store.ContactFieldByID(r.Context(), fieldID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	} else if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
		return
	}
	updated, err := h.store.UpdateContactFieldStatus(r.Context(), fieldID, status)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "状态修改失败", nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "状态修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactFieldHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/destroy#delete", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	fieldID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || fieldID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
		return
	}
	field, found, err := h.store.ContactFieldByID(r.Context(), fieldID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
		return
	}
	if field.IsSys == 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "系统初始化字段不可删除", nil)
		return
	}
	deleted, err := h.store.DeleteContactField(r.Context(), fieldID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactFieldHandler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/contactField/batchUpdate#put", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	updates, ok := h.parseContactFieldBatchUpdates(w, r, params)
	if !ok {
		return
	}
	destroyIDs, err := intSliceParam(params, "destroy")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必须为整数", nil)
		return
	}
	if err := h.store.BatchUpdateContactFields(r.Context(), updates, destroyIDs); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "批量修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactFieldHandler) writeFieldPivotIndex(w http.ResponseWriter, r *http.Request) {
	contactID, err := positiveQueryIntRequired(r, "contactId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	fields, err := h.store.ContactFieldsByStatusOrder(r.Context(), 1)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(fields) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}

	fieldIDs := make([]int, 0, len(fields))
	for _, field := range fields {
		fieldIDs = append(fieldIDs, field.ID)
	}
	pivots, err := h.store.ContactFieldPivotsByContactID(r.Context(), contactID, fieldIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(fields))
	for _, field := range fields {
		item := map[string]any{
			"contactFieldId":      field.ID,
			"name":                field.Label,
			"type":                field.Type,
			"options":             field.Options,
			"value":               "",
			"typeText":            contactFieldTypeText(field.Type),
			"contactFieldPivotId": "",
		}
		if field.Type == 2 {
			item["value"] = []any{}
		}
		if field.Type == 11 {
			item["pictureFlag"] = ""
		}
		if pivot, ok := pivots[field.ID]; ok {
			item["contactFieldPivotId"] = pivot.ID
			item["value"] = pivot.Value
			if field.Type == 11 && pivot.Value != "" {
				item["pictureFlag"] = h.fileFullURL(pivot.Value)
			}
			if field.Type == 2 {
				if pivot.Value == "" {
					item["value"] = []any{}
				} else {
					parts := strings.Split(pivot.Value, ",")
					values := make([]any, 0, len(parts))
					for _, part := range parts {
						values = append(values, part)
					}
					item["value"] = values
				}
			}
		}
		list = append(list, item)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *ContactFieldHandler) writeFieldPivotUpdate(w http.ResponseWriter, r *http.Request, params map[string]any, employeeID int, corpID int) {
	contactID, okInt, err := intParam(params, "contactId")
	if err != nil || !okInt || contactID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	rawPortrait, ok := params["userPortrait"]
	if !ok || isEmptyParam(rawPortrait) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改内容必传", nil)
		return
	}
	items, ok := contactFieldPivotUpdateItems(w, rawPortrait)
	if !ok {
		return
	}
	if len(items) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}

	content := "编辑用户画像："
	creates := make([]ContactFieldPivotCreate, 0)
	for _, item := range items {
		if item.PivotID <= 0 {
			creates = append(creates, ContactFieldPivotCreate{
				ContactID:      contactID,
				ContactFieldID: item.ContactFieldID,
				Value:          item.Value,
			})
			if item.Value != "" {
				content += item.Name + " "
			}
			continue
		}

		pivot, found, err := h.store.ContactFieldPivotByID(r.Context(), item.PivotID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if found && pivot.Value != item.Value {
			content += item.Name + " "
		}
		updated, err := h.store.UpdateContactFieldPivotValue(r.Context(), item.PivotID, item.Value)
		if err != nil || !updated {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "编辑用户画像失败", nil)
			return
		}
	}

	if content != "编辑用户画像：" {
		if err := h.store.CreateContactEmployeeTrack(r.Context(), ContactEmployeeTrackCreate{
			EmployeeID: employeeID,
			ContactID:  contactID,
			Content:    content,
			CorpID:     corpID,
			Event:      4,
		}); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "记录轨迹失败", nil)
			return
		}
	}
	if len(creates) > 0 {
		if err := h.store.CreateContactFieldPivots(r.Context(), creates); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "添加用户画像失败", nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactFieldHandler) parseContactFieldValues(w http.ResponseWriter, params map[string]any, existing ContactField, update bool) (ContactFieldWriteValues, bool) {
	label := stringParam(params, "label")
	if label == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "字段名称 必填", nil)
		return ContactFieldWriteValues{}, false
	}
	if len([]rune(label)) > 8 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "字段名称 最大长度为8", nil)
		return ContactFieldWriteValues{}, false
	}
	fieldType, ok, err := intParam(params, "type")
	if err != nil || !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "字段类型 必填", nil)
		return ContactFieldWriteValues{}, false
	}
	order, ok, err := intParam(params, "order")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "排序展示 必须为整数", nil)
		return ContactFieldWriteValues{}, false
	}
	if !ok {
		order = 0
	}
	status, ok, err := intParam(params, "status")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态必须为数字", nil)
		return ContactFieldWriteValues{}, false
	}
	if !ok {
		status = 1
		if update {
			status = existing.Status
		}
	}
	if status != 0 && status != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态必须在 0, 1中选择一个", nil)
		return ContactFieldWriteValues{}, false
	}
	optionsJSON, ok := contactFieldOptionsJSON(w, params["options"])
	if !ok {
		return ContactFieldWriteValues{}, false
	}

	values := ContactFieldWriteValues{
		Name:    contactFieldName(label),
		Label:   label,
		Type:    fieldType,
		Options: optionsJSON,
		Order:   order,
		Status:  status,
	}
	if update && existing.IsSys == 1 {
		values.Name = existing.Name
		values.Label = existing.Label
		values.Type = existing.Type
		values.Options = mustContactFieldOptionsJSON(existing.Options)
	}
	return values, true
}

type contactFieldPivotUpdateItem struct {
	PivotID        int
	ContactFieldID int
	Name           string
	Type           int
	Value          string
}

func contactFieldPivotUpdateItems(w http.ResponseWriter, raw any) ([]contactFieldPivotUpdateItem, bool) {
	var records []any
	switch typed := raw.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改内容必传", nil)
			return nil, false
		}
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		if err := decoder.Decode(&records); err != nil {
			return []contactFieldPivotUpdateItem{}, true
		}
	case []any:
		records = typed
	case []map[string]any:
		records = make([]any, 0, len(typed))
		for _, item := range typed {
			records = append(records, item)
		}
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改内容格式错误", nil)
		return nil, false
	}

	items := make([]contactFieldPivotUpdateItem, 0, len(records))
	for _, rawRecord := range records {
		record, ok := rawRecord.(map[string]any)
		if !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改内容格式错误", nil)
			return nil, false
		}
		pivotID, okInt, err := intParam(record, "contactFieldPivotId")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户画像ID必须为整数", nil)
			return nil, false
		}
		if !okInt {
			pivotID = 0
		}
		fieldID, okInt, err := intParam(record, "contactFieldId")
		if err != nil || !okInt || fieldID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户画像字段id必传", nil)
			return nil, false
		}
		fieldType, okInt, err := intParam(record, "type")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "字段类型必须为整数", nil)
			return nil, false
		}
		if !okInt {
			fieldType = 0
		}
		items = append(items, contactFieldPivotUpdateItem{
			PivotID:        pivotID,
			ContactFieldID: fieldID,
			Name:           stringParam(record, "name"),
			Type:           fieldType,
			Value:          contactFieldPivotValue(record["value"], fieldType),
		})
	}
	return items, true
}

func (h *ContactFieldHandler) parseContactFieldBatchUpdates(w http.ResponseWriter, r *http.Request, params map[string]any) ([]ContactFieldBatchUpdate, bool) {
	rawUpdates, ok := params["update"]
	if !ok || rawUpdates == nil {
		return []ContactFieldBatchUpdate{}, true
	}
	items, ok := rawUpdates.([]any)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "批量修改参数格式错误", nil)
		return nil, false
	}
	updates := make([]ContactFieldBatchUpdate, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "批量修改参数格式错误", nil)
			return nil, false
		}
		fieldID, okInt, err := intParam(item, "id")
		if err != nil || !okInt || fieldID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
			return nil, false
		}
		field, found, err := h.store.ContactFieldByID(r.Context(), fieldID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return nil, false
		}
		if !found {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
			return nil, false
		}
		values, ok := h.parseContactFieldValues(w, item, field, true)
		if !ok {
			return nil, false
		}
		updates = append(updates, ContactFieldBatchUpdate{ID: fieldID, Values: values})
	}
	return updates, true
}

func contactFieldPivotValue(raw any, fieldType int) string {
	if fieldType == 2 {
		values := stringValues(raw)
		if len(values) == 0 {
			return ""
		}
		return strings.Join(values, ",")
	}
	return valueAsString(raw)
}

func stringValues(raw any) []string {
	switch typed := raw.(type) {
	case nil:
		return []string{}
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			value := valueAsString(item)
			if value != "" {
				values = append(values, value)
			}
		}
		return values
	case []string:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(item)
			if value != "" {
				values = append(values, value)
			}
		}
		return values
	case string:
		if strings.TrimSpace(typed) == "" {
			return []string{}
		}
		parts := strings.Split(typed, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value != "" {
				values = append(values, value)
			}
		}
		return values
	default:
		value := valueAsString(raw)
		if value == "" {
			return []string{}
		}
		return []string{value}
	}
}

func valueAsString(raw any) string {
	switch typed := raw.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case []string:
		if len(typed) == 0 {
			return ""
		}
		return strings.TrimSpace(typed[0])
	case []any:
		if len(typed) == 0 {
			return ""
		}
		return valueAsString(typed[0])
	default:
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}

func isEmptyParam(raw any) bool {
	if raw == nil {
		return true
	}
	if text, ok := raw.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func contactFieldOptionsJSON(w http.ResponseWriter, value any) (string, bool) {
	if value == nil {
		return "[]", true
	}
	if raw, ok := value.(string); ok {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "[]", true
		}
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "选项内容 格式错误: 必须为数组", nil)
			return "", false
		}
		if _, ok := decoded.([]any); !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "选项内容 格式错误: 必须为数组", nil)
			return "", false
		}
		return raw, true
	}
	if _, ok := value.([]any); !ok {
		if _, ok := value.([]string); !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "选项内容 格式错误: 必须为数组", nil)
			return "", false
		}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "选项内容 格式错误: 必须为数组", nil)
		return "", false
	}
	return string(payload), true
}

func mustContactFieldOptionsJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func contactFieldName(label string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		}
	}
	if builder.Len() > 0 {
		return builder.String()
	}
	sum := sha1.Sum([]byte(label))
	return "field_" + fmt.Sprintf("%x", sum[:4])
}

func (h *ContactFieldHandler) resolveSidebarAccess(w http.ResponseWriter, r *http.Request) (SidebarEmployee, bool) {
	if h.sidebar == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "sidebar employee resolver not configured", nil)
		return SidebarEmployee{}, false
	}
	employeeID, err := h.sidebar.UserID(r)
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return SidebarEmployee{}, false
	}
	employee, found, err := h.store.SidebarEmployeeByID(r.Context(), employeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return SidebarEmployee{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "employee not found", nil)
		return SidebarEmployee{}, false
	}
	return employee, true
}

func (h *ContactFieldHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, false
		}
	}
	loginInfo, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	return userID, user, LoginCorpInfo(loginInfo), true
}

func (h *ContactFieldHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func contactFieldPayload(field ContactField) map[string]any {
	return map[string]any{
		"id":       field.ID,
		"name":     field.Name,
		"label":    field.Label,
		"type":     field.Type,
		"options":  field.Options,
		"status":   field.Status,
		"order":    field.Order,
		"isSys":    field.IsSys,
		"typeText": contactFieldTypeText(field.Type),
	}
}

func contactFieldTypeText(fieldType int) string {
	switch fieldType {
	case 0:
		return "文本"
	case 1:
		return "单选"
	case 2:
		return "多选"
	case 3:
		return "下拉"
	case 4:
		return "文件"
	case 5:
		return "文本域"
	case 6:
		return "日期"
	case 7:
		return "日期时间"
	case 8:
		return "数字"
	case 9:
		return "手机号"
	case 10:
		return "邮箱"
	case 11:
		return "图片"
	default:
		return ""
	}
}

func ParseContactFieldOptions(raw string) any {
	if raw == "" {
		return []any{}
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	if value == nil {
		return []any{}
	}
	return value
}
