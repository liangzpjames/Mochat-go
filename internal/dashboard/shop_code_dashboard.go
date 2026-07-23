package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ShopCodeFilter struct {
	CorpID  int
	Type    int
	Status  int
	Name    string
	City    string
	Page    int
	PerPage int
}

type ShopCodeItem struct {
	ID                int
	Name              string
	Type              int
	EmployeeRaw       string
	EmployeeQRCodeRaw string
	QWCodeRaw         string
	SearchKeyword     string
	Address           string
	Country           string
	Province          string
	City              string
	District          string
	Lat               string
	Lng               string
	Status            int
	TenantID          int
	CorpID            int
	CreateUserID      int
	CreatedAt         string
	UpdatedAt         string
}

type ShopCodePage struct {
	Items     []ShopCodeItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type ShopCodeWrite struct {
	CorpID            int
	CreateUserID      int
	Name              string
	HasName           bool
	Type              int
	HasType           bool
	EmployeeRaw       string
	HasEmployee       bool
	EmployeeQRCodeRaw string
	HasEmployeeQRCode bool
	QWCodeRaw         string
	HasQWCode         bool
	SearchKeyword     string
	HasSearchKeyword  bool
	Address           string
	HasAddress        bool
	Country           string
	HasCountry        bool
	Province          string
	HasProvince       bool
	City              string
	HasCity           bool
	District          string
	HasDistrict       bool
	Lat               string
	HasLat            bool
	Lng               string
	HasLng            bool
	Status            int
	HasStatus         bool
}

type ShopCodePageSetting struct {
	ID           int
	Type         int
	Title        string
	ShowType     int
	DefaultRaw   string
	Poster       string
	AutoPass     int
	TenantID     int
	CorpID       int
	CreateUserID int
	CreatedAt    string
	UpdatedAt    string
}

type ShopCodePageSettingWrite struct {
	CorpID       int
	CreateUserID int
	Type         int
	Title        string
	ShowType     int
	DefaultRaw   string
	Poster       string
	AutoPass     int
}

type ShopCodeAddressSuggestion struct {
	ID            int
	Name          string
	Address       string
	SearchKeyword string
	Country       string
	Province      string
	City          string
	District      string
	Lat           string
	Lng           string
}

type ShopCodeCity struct {
	Country  string
	Province string
	City     string
	District string
}

type ShopCodeOverview struct {
	ShopTotal   int
	OpenTotal   int
	CloseTotal  int
	RecordTotal int
}

type ShopCodeRecordFilter struct {
	CorpID  int
	Type    int
	ShopID  int
	Page    int
	PerPage int
}

type ShopCodeRecordItem struct {
	ID        int
	Type      int
	CorpID    int
	ShopID    int
	ShopName  string
	CreatedAt string
}

type ShopCodeRecordPage struct {
	Items     []ShopCodeRecordItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type ShopCodeShopStat struct {
	ShopCodeItem
	RecordTotal int
}

type ShopCodeShopStatPage struct {
	Items     []ShopCodeShopStat
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type ShopCodeStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	ShopCodePage(ctx context.Context, filter ShopCodeFilter) (ShopCodePage, error)
	ShopCodeByID(ctx context.Context, corpID int, id int) (ShopCodeItem, bool, error)
	CreateShopCode(ctx context.Context, values ShopCodeWrite) (int, error)
	UpdateShopCode(ctx context.Context, corpID int, id int, values ShopCodeWrite) (bool, error)
	UpdateShopCodeStatus(ctx context.Context, corpID int, id int, status int) (bool, error)
	DeleteShopCode(ctx context.Context, corpID int, id int) (bool, error)
	ShopCodePageSetting(ctx context.Context, corpID int, codeType int) (ShopCodePageSetting, bool, error)
	UpsertShopCodePageSetting(ctx context.Context, values ShopCodePageSettingWrite) (int, error)
	ShopCodeAddressSuggestions(ctx context.Context, corpID int, keyword string, city string) ([]ShopCodeAddressSuggestion, error)
	ShopCodeCities(ctx context.Context, corpID int, keyword string) ([]ShopCodeCity, error)
	ShopCodeOverview(ctx context.Context, corpID int, codeType int) (ShopCodeOverview, error)
	ShopCodeRecordPage(ctx context.Context, filter ShopCodeRecordFilter) (ShopCodeRecordPage, error)
	ShopCodeShopStatPage(ctx context.Context, filter ShopCodeFilter) (ShopCodeShopStatPage, error)
}

type ShopCodeHandler struct {
	store            ShopCodeStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	operationBaseURL string
}

func NewShopCodeHandler(store ShopCodeStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, operationBaseURL string) *ShopCodeHandler {
	return &ShopCodeHandler{
		store:            store,
		cache:            cache,
		resolver:         resolver,
		authorizer:       authorizer,
		operationBaseURL: strings.TrimRight(strings.TrimSpace(operationBaseURL), "/"),
	}
}

func (h *ShopCodeHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.ShopCodePage(r.Context(), ShopCodeFilter{
		CorpID:  corpID,
		Type:    positiveQueryInt(r, "type", 0),
		Status:  shopCodeStatusQuery(r),
		Name:    shopCodeQueryName(r),
		City:    strings.TrimSpace(r.URL.Query().Get("city")),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, shopCodePayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *ShopCodeHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/store#post")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	values, err := shopCodeWriteFromParams(params, corpID, userID, true)
	if err != nil {
		if isBadRequestError(err) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricShopCodes, 1) {
		return
	}
	id, err := h.store.CreateShopCode(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricShopCodes); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *ShopCodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/shopCode/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := shopCodeIDFromParams(params)
		if err != nil {
			return nil, err
		}
		values, err := shopCodeWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.UpdateShopCode(ctx, corpID, id, values)
		return nil, sopRequireFound(ok, err, "门店活码不存在")
	})
}

func (h *ShopCodeHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	id, err := shopCodeIDFromParams(params)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	deleted, err := h.store.DeleteShopCode(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "门店活码不存在", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricShopCodes); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ShopCodeHandler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/info#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	id := shopCodeIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "shopCodeId required", nil)
		return
	}
	item, found, err := h.store.ShopCodeByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "门店活码不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", shopCodePayload(item))
}

func (h *ShopCodeHandler) Status(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/shopCode/status#put", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := shopCodeIDFromParams(params)
		if err != nil {
			return nil, err
		}
		status, found, err := sopStateParam(params, -1)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("status required")
		}
		ok, err := h.store.UpdateShopCodeStatus(ctx, corpID, id, status)
		return nil, sopRequireFound(ok, err, "门店活码不存在")
	})
}

func (h *ShopCodeHandler) SearchCity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/searchCity#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	cities, err := h.store.ShopCodeCities(r.Context(), corpID, shopCodeQueryName(r))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(cities))
	for _, city := range cities {
		list = append(list, map[string]any{
			"country":  city.Country,
			"province": city.Province,
			"city":     city.City,
			"district": city.District,
			"name":     shopCodeFirstNonEmpty(city.District, city.City, city.Province, city.Country),
			"value":    shopCodeFirstNonEmpty(city.City, city.Province, city.District, city.Country),
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *ShopCodeHandler) AddressKeyWordList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/addressKeyWordList#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	items, err := h.store.ShopCodeAddressSuggestions(r.Context(), corpID, shopCodeQueryName(r), strings.TrimSpace(r.URL.Query().Get("city")))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(items))
	for _, item := range items {
		list = append(list, map[string]any{
			"id":             item.ID,
			"name":           shopCodeFirstNonEmpty(item.Name, item.SearchKeyword, item.Address),
			"title":          shopCodeFirstNonEmpty(item.Name, item.SearchKeyword, item.Address),
			"address":        item.Address,
			"searchKeyword":  item.SearchKeyword,
			"search_keyword": item.SearchKeyword,
			"country":        item.Country,
			"province":       item.Province,
			"city":           item.City,
			"district":       item.District,
			"lat":            item.Lat,
			"lng":            item.Lng,
			"location":       map[string]any{"lat": item.Lat, "lng": item.Lng},
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *ShopCodeHandler) Location(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/location#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	item := ShopCodeItem{
		Address:  strings.TrimSpace(r.URL.Query().Get("address")),
		City:     strings.TrimSpace(r.URL.Query().Get("city")),
		District: strings.TrimSpace(r.URL.Query().Get("district")),
		Lat:      shopCodeFirstNonEmpty(strings.TrimSpace(r.URL.Query().Get("lat")), strings.TrimSpace(r.URL.Query().Get("latitude"))),
		Lng:      shopCodeFirstNonEmpty(strings.TrimSpace(r.URL.Query().Get("lng")), strings.TrimSpace(r.URL.Query().Get("longitude"))),
	}
	if id := shopCodeIDFromQuery(r); id > 0 {
		stored, found, err := h.store.ShopCodeByID(r.Context(), corpID, id)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if found {
			item = stored
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"address":   item.Address,
		"country":   item.Country,
		"province":  item.Province,
		"city":      item.City,
		"district":  item.District,
		"lat":       item.Lat,
		"lng":       item.Lng,
		"latitude":  item.Lat,
		"longitude": item.Lng,
		"location":  map[string]any{"lat": item.Lat, "lng": item.Lng},
	})
}

func (h *ShopCodeHandler) Share(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/share#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	id := shopCodeIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "shopCodeId required", nil)
		return
	}
	item, found, err := h.store.ShopCodeByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "门店活码不存在", nil)
		return
	}
	target := "/shopCode?id=" + strconv.Itoa(item.ID) + "&type=" + strconv.Itoa(item.Type)
	base := h.operationBaseURL
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	link := base + "/auth/shopCode?id=" + strconv.Itoa(item.ID) + "&target=" + url.QueryEscape(target)
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":       item.ID,
		"type":     item.Type,
		"name":     item.Name,
		"url":      link,
		"link":     link,
		"shareUrl": link,
		"target":   target,
	})
}

func (h *ShopCodeHandler) PageInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/pageInfo#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	codeType := positiveQueryInt(r, "type", 1)
	item, found, err := h.store.ShopCodePageSetting(r.Context(), corpID, codeType)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		item = defaultShopCodePageSetting(corpID, codeType)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", shopCodePageSettingPayload(item))
}

func (h *ShopCodeHandler) PageSet(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/shopCode/pageSet#post", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		values, err := shopCodePageSettingWriteFromParams(params, corpID, userID)
		if err != nil {
			return nil, err
		}
		id, err := h.store.UpsertShopCodePageSetting(ctx, values)
		if err != nil {
			return nil, err
		}
		return []any{id}, nil
	})
}

func (h *ShopCodeHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/show#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	overview, err := h.store.ShopCodeOverview(r.Context(), corpID, positiveQueryInt(r, "type", 0))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"shopTotal":    overview.ShopTotal,
		"shop_total":   overview.ShopTotal,
		"openTotal":    overview.OpenTotal,
		"open_total":   overview.OpenTotal,
		"closeTotal":   overview.CloseTotal,
		"close_total":  overview.CloseTotal,
		"recordTotal":  overview.RecordTotal,
		"record_total": overview.RecordTotal,
	})
}

func (h *ShopCodeHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/showContact#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.ShopCodeRecordPage(r.Context(), ShopCodeRecordFilter{
		CorpID:  corpID,
		Type:    positiveQueryInt(r, "type", 0),
		ShopID:  shopCodeIDFromQuery(r),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":         item.ID,
			"type":       item.Type,
			"corpId":     item.CorpID,
			"corp_id":    item.CorpID,
			"shopId":     item.ShopID,
			"shop_id":    item.ShopID,
			"shopName":   item.ShopName,
			"shop_name":  item.ShopName,
			"createdAt":  item.CreatedAt,
			"created_at": item.CreatedAt,
		})
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *ShopCodeHandler) ShowShop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/shopCode/showShop#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.ShopCodeShopStatPage(r.Context(), ShopCodeFilter{
		CorpID:  corpID,
		Type:    positiveQueryInt(r, "type", 0),
		Status:  shopCodeStatusQuery(r),
		Name:    shopCodeQueryName(r),
		City:    strings.TrimSpace(r.URL.Query().Get("city")),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, stat := range page.Items {
		payload := shopCodePayload(stat.ShopCodeItem)
		payload["recordTotal"] = stat.RecordTotal
		payload["record_total"] = stat.RecordTotal
		list = append(list, payload)
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *ShopCodeHandler) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/shopCode/updateEmployee#post", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := shopCodeIDFromParams(params)
		if err != nil {
			return nil, err
		}
		raw, found, err := sopRawJSONParam(params, "employee", "employees", "employeeIds", "employee_ids", "employeeList")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("employee required")
		}
		ok, err := h.store.UpdateShopCode(ctx, corpID, id, ShopCodeWrite{CorpID: corpID, CreateUserID: userID, EmployeeRaw: raw, HasEmployee: true})
		return nil, sopRequireFound(ok, err, "门店活码不存在")
	})
}

func (h *ShopCodeHandler) UpdateQRCode(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/shopCode/updateQrcode#post", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := shopCodeIDFromParams(params)
		if err != nil {
			return nil, err
		}
		values := ShopCodeWrite{CorpID: corpID, CreateUserID: userID}
		if raw, found, err := sopRawJSONParam(params, "employeeQrcode", "employee_qrcode", "employeeQRCode", "qrcode", "qrCode", "qr_code"); err != nil {
			return nil, err
		} else if found {
			values.EmployeeQRCodeRaw = raw
			values.HasEmployeeQRCode = true
		}
		if raw, found, err := sopRawJSONParam(params, "qwCode", "qw_code", "code", "roomCode", "room_code"); err != nil {
			return nil, err
		} else if found {
			values.QWCodeRaw = raw
			values.HasQWCode = true
		}
		if !values.HasEmployeeQRCode && !values.HasQWCode {
			return nil, badRequestError("qrcode required")
		}
		ok, err := h.store.UpdateShopCode(ctx, corpID, id, values)
		return nil, sopRequireFound(ok, err, "门店活码不存在")
	})
}

func (h *ShopCodeHandler) BatchContactTags(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/shopCode/batchContactTags#put", func(_ context.Context, _ int, _ int, params map[string]any) (any, error) {
		tagIDsRaw, found, err := sopRawJSONParam(params, "tagIds", "tag_ids", "tags", "contactTags", "contact_tags")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("tagIds required")
		}
		return map[string]any{
			"tagIds":  sopJSONPayload(tagIDsRaw),
			"tag_ids": sopJSONPayload(tagIDsRaw),
			"applied": 0,
		}, nil
	})
}

func (h *ShopCodeHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	data, err := action(r.Context(), userID, corpID, params)
	if err != nil {
		if isBadRequestError(err) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if data == nil {
		data = []any{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *ShopCodeHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	employeeID := loginInfo.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, loginInfo, access, true
}

func (h *ShopCodeHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
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

func shopCodeWriteFromParams(params map[string]any, corpID int, userID int, requireCreateFields bool) (ShopCodeWrite, error) {
	values := ShopCodeWrite{CorpID: corpID, CreateUserID: userID}
	if name, found := sopStringParam(params, "name", "shopName", "title"); found {
		values.Name = name
		values.HasName = true
	}
	if codeType, found, err := intParam(params, "type"); err != nil {
		return ShopCodeWrite{}, badRequestError("type invalid")
	} else if found {
		values.Type = codeType
		values.HasType = true
	}
	if requireCreateFields {
		if strings.TrimSpace(values.Name) == "" {
			return ShopCodeWrite{}, badRequestError("name required")
		}
		if values.Type <= 0 {
			return ShopCodeWrite{}, badRequestError("type required")
		}
	}
	if raw, found, err := sopRawJSONParam(params, "employee", "employees", "employeeIds", "employee_ids", "employeeList"); err != nil {
		return ShopCodeWrite{}, err
	} else if found {
		values.EmployeeRaw = raw
		values.HasEmployee = true
	}
	if raw, found, err := sopRawJSONParam(params, "employeeQrcode", "employee_qrcode", "employeeQRCode", "qrcode", "qrCode", "qr_code"); err != nil {
		return ShopCodeWrite{}, err
	} else if found {
		values.EmployeeQRCodeRaw = raw
		values.HasEmployeeQRCode = true
	}
	if raw, found, err := sopRawJSONParam(params, "qwCode", "qw_code", "code", "roomCode", "room_code", "autoPullCode"); err != nil {
		return ShopCodeWrite{}, err
	} else if found {
		values.QWCodeRaw = raw
		values.HasQWCode = true
	}
	shopCodeStringField(params, &values.SearchKeyword, &values.HasSearchKeyword, "searchKeyword", "search_keyword", "keyword", "keyWords")
	shopCodeStringField(params, &values.Address, &values.HasAddress, "address")
	shopCodeStringField(params, &values.Country, &values.HasCountry, "country")
	shopCodeStringField(params, &values.Province, &values.HasProvince, "province")
	shopCodeStringField(params, &values.City, &values.HasCity, "city")
	shopCodeStringField(params, &values.District, &values.HasDistrict, "district")
	shopCodeStringField(params, &values.Lat, &values.HasLat, "lat", "latitude")
	shopCodeStringField(params, &values.Lng, &values.HasLng, "lng", "longitude")
	defaultStatus := -1
	if requireCreateFields {
		defaultStatus = 1
	}
	if status, found, err := sopStateParam(params, defaultStatus); err != nil {
		return ShopCodeWrite{}, err
	} else if found {
		values.Status = status
		values.HasStatus = true
	}
	return values, nil
}

func shopCodePageSettingWriteFromParams(params map[string]any, corpID int, userID int) (ShopCodePageSettingWrite, error) {
	codeType, _, err := intParam(params, "type")
	if err != nil || codeType <= 0 {
		return ShopCodePageSettingWrite{}, badRequestError("type required")
	}
	values := ShopCodePageSettingWrite{CorpID: corpID, CreateUserID: userID, Type: codeType, Title: "门店活码", ShowType: 1, DefaultRaw: "{}", AutoPass: 0}
	if title, found := sopStringParam(params, "title", "name"); found && strings.TrimSpace(title) != "" {
		values.Title = title
	}
	if showType, found, err := intParam(params, "showType"); err != nil {
		return ShopCodePageSettingWrite{}, badRequestError("showType invalid")
	} else if found && showType > 0 {
		values.ShowType = showType
	}
	if raw, found, err := sopRawJSONParam(params, "default", "defaultStyle", "default_style", "setting", "settings"); err != nil {
		return ShopCodePageSettingWrite{}, err
	} else if found {
		values.DefaultRaw = raw
	}
	if poster, found := sopStringParam(params, "poster", "posterUrl", "poster_url"); found {
		values.Poster = poster
	}
	if autoPass, found, err := intParam(params, "autoPass"); err != nil {
		return ShopCodePageSettingWrite{}, badRequestError("autoPass invalid")
	} else if found {
		values.AutoPass = autoPass
	}
	return values, nil
}

func shopCodeStringField(params map[string]any, target *string, present *bool, keys ...string) {
	for _, key := range keys {
		if _, ok := params[key]; !ok {
			continue
		}
		*target = stringParam(params, key)
		*present = true
		return
	}
}

func shopCodeIDFromParams(params map[string]any) (int, error) {
	id, _, err := firstPositiveIntParam(params, "shopCodeId", "shop_code_id", "shopId", "shop_id", "id")
	if err != nil || id <= 0 {
		return 0, badRequestError("shopCodeId required")
	}
	return id, nil
}

func shopCodeIDFromQuery(r *http.Request) int {
	for _, key := range []string{"shopCodeId", "shop_code_id", "shopId", "shop_id", "id"} {
		if id := positiveQueryInt(r, key, 0); id > 0 {
			return id
		}
	}
	return 0
}

func shopCodeStatusQuery(r *http.Request) int {
	for _, key := range []string{"status", "state"} {
		if raw := strings.TrimSpace(r.URL.Query().Get(key)); raw != "" {
			value, err := strconv.Atoi(raw)
			if err == nil {
				return value
			}
		}
	}
	return -1
}

func shopCodeQueryName(r *http.Request) string {
	for _, key := range []string{"name", "searchKeyword", "search_keyword", "keyword", "keyWords"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func shopCodePayload(item ShopCodeItem) map[string]any {
	employee := sopJSONPayload(item.EmployeeRaw)
	employeeQRCode := sopJSONPayload(item.EmployeeQRCodeRaw)
	qwCode := sopJSONPayload(item.QWCodeRaw)
	return map[string]any{
		"id":                item.ID,
		"shopCodeId":        item.ID,
		"shop_code_id":      item.ID,
		"name":              item.Name,
		"type":              item.Type,
		"employee":          employee,
		"employeeRaw":       item.EmployeeRaw,
		"employee_qrcode":   employeeQRCode,
		"employeeQrcode":    employeeQRCode,
		"employeeQRCode":    employeeQRCode,
		"employeeQrcodeRaw": item.EmployeeQRCodeRaw,
		"qrcode":            employeeQRCode,
		"qw_code":           qwCode,
		"qwCode":            qwCode,
		"qwCodeRaw":         item.QWCodeRaw,
		"searchKeyword":     item.SearchKeyword,
		"search_keyword":    item.SearchKeyword,
		"address":           item.Address,
		"country":           item.Country,
		"province":          item.Province,
		"city":              item.City,
		"district":          item.District,
		"lat":               item.Lat,
		"lng":               item.Lng,
		"status":            item.Status,
		"state":             item.Status,
		"tenantId":          item.TenantID,
		"tenant_id":         item.TenantID,
		"corpId":            item.CorpID,
		"corp_id":           item.CorpID,
		"createUserId":      item.CreateUserID,
		"create_user_id":    item.CreateUserID,
		"createdAt":         item.CreatedAt,
		"created_at":        item.CreatedAt,
		"updatedAt":         item.UpdatedAt,
		"updated_at":        item.UpdatedAt,
	}
}

func shopCodePageSettingPayload(item ShopCodePageSetting) map[string]any {
	defaultValue := sopJSONPayload(item.DefaultRaw)
	return map[string]any{
		"id":             item.ID,
		"type":           item.Type,
		"title":          item.Title,
		"showType":       item.ShowType,
		"show_type":      item.ShowType,
		"default":        defaultValue,
		"defaultRaw":     item.DefaultRaw,
		"poster":         item.Poster,
		"autoPass":       item.AutoPass,
		"auto_pass":      item.AutoPass,
		"tenantId":       item.TenantID,
		"tenant_id":      item.TenantID,
		"corpId":         item.CorpID,
		"corp_id":        item.CorpID,
		"createUserId":   item.CreateUserID,
		"create_user_id": item.CreateUserID,
		"createdAt":      item.CreatedAt,
		"created_at":     item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
		"updated_at":     item.UpdatedAt,
	}
}

func defaultShopCodePageSetting(corpID int, codeType int) ShopCodePageSetting {
	return ShopCodePageSetting{
		Type:       codeType,
		Title:      "门店活码",
		ShowType:   1,
		DefaultRaw: "{}",
		Poster:     "",
		AutoPass:   0,
		CorpID:     corpID,
	}
}

func shopCodeFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mustShopCodeJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
