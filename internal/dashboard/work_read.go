package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type WorkDepartment struct {
	ID             int               `json:"id"`
	WXDepartmentID int               `json:"wxDepartmentId"`
	CorpID         int               `json:"corpId"`
	Name           string            `json:"name"`
	ParentID       int               `json:"parentId"`
	WXParentID     int               `json:"wxParentid"`
	Order          int               `json:"order"`
	Level          int               `json:"level"`
	Path           string            `json:"path"`
	CreatedAt      string            `json:"createdAt,omitempty"`
	UpdatedAt      string            `json:"updatedAt,omitempty"`
	Son            []*WorkDepartment `json:"son,omitempty"`
}

type WorkDepartmentEmployee struct {
	ID         int    `json:"id"`
	EmployeeID int    `json:"employeeId"`
	Name       string `json:"name"`
	WXUserID   string `json:"wxUserId"`
	Avatar     string `json:"avatar"`
}

type WorkDepartmentMember struct {
	EmployeeID     int    `json:"employeeId"`
	DepartmentID   int    `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
	EmployeeName   string `json:"employeeName"`
}

type WorkDepartmentPhoneOption struct {
	CorpID             int    `json:"corpId"`
	WorkDepartmentID   int    `json:"workDepartmentId"`
	WorkDepartmentName string `json:"workDepartmentName"`
}

type WorkDepartmentEmployeeListFilter struct {
	CorpID       int
	DepartmentID int
	Page         int
	PerPage      int
}

type WorkDepartmentEmployeeListItem struct {
	EmployeeID   int
	EmployeeName string
	Phone        string
	RoleName     string
}

type WorkDepartmentEmployeePage struct {
	Items     []WorkDepartmentEmployeeListItem
	Total     int
	TotalPage int
}

type WorkEmployeeIndexFilter struct {
	CorpIDs             []int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
	Name                string
	Status              int
	ContactAuth         string
	Page                int
	PerPage             int
}

type WorkEmployeeIndexItem struct {
	ID                int
	Name              string
	ThumbAvatar       string
	Status            int
	ContactAuth       int
	WXUserID          string
	CorpID            int
	Gender            int
	MessageNums       int
	SendMessageNums   int
	ReplyMessageRatio string
	AddNums           int
	ApplyNums         int
	InvalidContact    int
	AverageReply      int
}

type WorkEmployeeIndexPage struct {
	Items     []WorkEmployeeIndexItem
	Total     int
	TotalPage int
}

type WorkContactTagGroup struct {
	ID        int
	WXGroupID string
	GroupName string
}

type WorkContactTagFilter struct {
	CorpIDs []int
	GroupID *int
	Page    int
	PerPage int
}

type WorkContactTagItem struct {
	ID         int
	Name       string
	ContactNum int
}

type WorkContactTagPage struct {
	Items     []WorkContactTagItem
	Total     int
	TotalPage int
	PerPage   int
}

type WorkContactTagDetail struct {
	TagID          int
	WXContactTagID string
	TagName        string
	GroupID        int
}

type WorkContactTagOption struct {
	ID   int
	Name string
}

type WorkContactTagListTag struct {
	ID                int
	WXContactTagID    string
	Name              string
	ContactTagGroupID int
}

type WorkContactTagListGroup struct {
	ID        int
	WXGroupID string
	GroupName string
	Tags      []WorkContactTagListTag
}

type WorkContactTagGroupWrite struct {
	CorpID    int
	GroupName string
}

type WorkContactTagWrite struct {
	CorpID   int
	GroupID  int
	TagNames []string
}

type WorkContactDetail struct {
	ID     int
	Name   string
	Avatar string
	CorpID int
}

type WorkContactShow struct {
	Name         string
	Avatar       string
	Gender       int
	BusinessNo   string
	Remark       string
	Description  string
	Tags         []WorkContactShowTag
	RoomNames    []string
	EmployeeName []string
}

type WorkContactShowTag struct {
	TagID   int
	TagName string
}

type WorkContactIndexFilter struct {
	CorpID            int
	EmployeeIDs       []int
	RestrictEmployees bool
	CurrentEmployeeID int
	Remark            string
	AddWay            *int
	StartTime         string
	EndTime           string
	KeyWords          string
	BusinessNo        string
	Gender            *int
	FieldID           *int
	FieldValue        string
	RoomIDs           []int
	GroupNum          *int
	Page              int
	PerPage           int
}

type WorkContactIndexPage struct {
	Items           []WorkContactIndexItem
	Total           int
	TotalPage       int
	PerPage         int
	SyncContactTime string
	FilterNoData    bool
	EmptyData       bool
}

type WorkContactIndexItem struct {
	ID           int
	EmployeeID   int
	ContactID    int
	Remark       string
	CreateTime   string
	AddWay       int
	BusinessNo   string
	Name         string
	Avatar       string
	Gender       int
	RoomName     []string
	EmployeeName string
	Tag          []string
	IsContact    int
}

type WorkContactLossFilter struct {
	CorpID            int
	EmployeeIDs       []int
	RestrictEmployees bool
	Page              int
	PerPage           int
}

type WorkContactLossPage struct {
	Items     []WorkContactLossItem
	Total     int
	TotalPage int
	PerPage   int
	NoData    bool
}

type WorkContactLossItem struct {
	ID           int
	EmployeeID   int
	ContactID    int
	DeletedAt    string
	Avatar       string
	Name         string
	Tag          []string
	EmployeeName string
	Remark       string
}

type WorkContactRoomFilter struct {
	WorkRoomID int
	CorpID     int
	Status     *int
	Name       string
	StartTime  string
	EndTime    string
	Page       int
	PerPage    int
}

type WorkContactRoomPage struct {
	MemberNum  int
	OutRoomNum int
	Items      []WorkContactRoomItem
	Total      int
	TotalPage  int
	PerPage    int
}

type WorkContactRoomItem struct {
	WorkContactRoomID int
	Name              string
	Avatar            string
	IsOwner           int
	JoinTime          string
	OutRoomTime       string
	OtherRooms        []string
	JoinScene         int
	Type              int
	ContactID         int
	EmployeeID        int
	ContactEmployeeID int
}

type WorkRoomOptionFilter struct {
	CorpIDs     []int
	Name        string
	RoomGroupID *int
}

type WorkRoomOptionPage struct {
	Total int
	Items []WorkRoomOption
}

type WorkRoomOption struct {
	RoomID     int
	RoomName   string
	RoomMax    int
	CurrentNum int
}

type WorkRoomIndexFilter struct {
	CorpID        int
	OwnerIDs      []int
	RestrictOwner bool
	RoomGroupID   *int
	Name          string
	Status        *int
	StartTime     string
	EndTime       string
	Page          int
	PerPage       int
}

type WorkRoomIndexPage struct {
	Items     []WorkRoomIndexItem
	Total     int
	TotalPage int
	PerPage   int
}

type WorkRoomIndexItem struct {
	WorkRoomID int
	MemberNum  int
	RoomName   string
	OwnerID    int
	OwnerName  string
	RoomGroup  string
	Status     int
	InRoomNum  int
	OutRoomNum int
	Notice     string
	CreateTime string
}

type WorkRoomMemberStat struct {
	Status   int
	JoinTime string
	OutTime  string
}

type WorkRoomStatisticRequest struct {
	WorkRoomID int
	Type       int
	StartTime  string
	EndTime    string
	Page       int
	PerPage    int
}

type WorkRoomStatisticPoint struct {
	Time     string
	AddNum   int
	OutNum   int
	Total    int
	OutTotal int
}

type ContactEmployeeTrack struct {
	ID        int
	Content   string
	CreatedAt string
}

type ContactProcessStatus struct {
	ID   int
	Name string
}

type ContactProcessStatusUpdate struct {
	ContactID  int
	StatusID   int
	EmployeeID int
	CorpID     int
	Event      int
	Content    string
}

type workContactSourceOption struct {
	AddWay     int
	AddWayText string
}

const contactEmployeeTrackEventProcessStatus = 5

type WorkReadStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
	CorpIDsByUser(ctx context.Context, userID int) ([]int, error)
	WorkEmployeeSyncTime(ctx context.Context, corpIDs []int) (string, error)
	WorkDepartmentsByCorp(ctx context.Context, corpID int, search string) ([]WorkDepartment, error)
	ActiveWorkEmployeesByCorp(ctx context.Context, corpID int, search string) ([]WorkDepartmentEmployee, error)
	WorkDepartmentMembers(ctx context.Context, corpID int, departmentIDs []int) ([]WorkDepartmentMember, error)
	WorkDepartmentsByEmployeeMobile(ctx context.Context, corpID int, phone string) ([]WorkDepartmentPhoneOption, error)
	WorkDepartmentEmployeePage(ctx context.Context, filter WorkDepartmentEmployeeListFilter) (WorkDepartmentEmployeePage, error)
	WorkEmployeeIndexPage(ctx context.Context, filter WorkEmployeeIndexFilter) (WorkEmployeeIndexPage, error)
	WorkEmployeeSyncCredentials(ctx context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error)
	SyncWorkEmployees(ctx context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error)
	WorkContactSyncEmployees(ctx context.Context, corpID int) ([]WorkContactSyncEmployee, error)
	SyncWorkContacts(ctx context.Context, corpID int, bundles []WorkContactSyncEmployeeContacts) (WorkContactSyncResult, error)
	WorkContactTagGroupsByCorp(ctx context.Context, corpIDs []int) ([]WorkContactTagGroup, error)
	WorkContactTagGroupByID(ctx context.Context, groupID int) (WorkContactTagGroup, bool, error)
	WorkContactTagSyncTime(ctx context.Context, corpIDs []int) (string, error)
	WorkContactTagPage(ctx context.Context, filter WorkContactTagFilter) (WorkContactTagPage, error)
	WorkContactTagByID(ctx context.Context, tagID int) (WorkContactTagDetail, bool, error)
	WorkContactTags(ctx context.Context, corpIDs []int, groupID *int) ([]WorkContactTagOption, error)
	WorkContactTagList(ctx context.Context, corpIDs []int, name string) ([]WorkContactTagListGroup, error)
	WorkContactTagGroupNameExists(ctx context.Context, corpID int, groupName string, excludeGroupID int) (bool, error)
	CreateWorkContactTagGroup(ctx context.Context, values WorkContactTagGroupWrite) (int, error)
	UpdateWorkContactTagGroup(ctx context.Context, corpID int, groupID int, groupName string) (bool, error)
	UpdateWorkContactTagGroupWXID(ctx context.Context, corpID int, groupID int, wxGroupID string) (bool, error)
	DeleteWorkContactTagGroupCascade(ctx context.Context, corpID int, groupID int) (bool, error)
	WorkContactTagNamesExist(ctx context.Context, corpID int, groupID int, names []string, excludeTagIDs []int) (bool, error)
	CreateWorkContactTags(ctx context.Context, values WorkContactTagWrite) error
	UpdateWorkContactTagWXIDsByName(ctx context.Context, corpID int, groupID int, tagWXIDs map[string]string) error
	UpdateWorkContactTag(ctx context.Context, corpID int, tagID int, groupID int, tagName string) (bool, error)
	DeleteWorkContactTags(ctx context.Context, corpID int, tagIDs []int) (bool, error)
	MoveWorkContactTags(ctx context.Context, corpID int, tagIDs []int, groupID int) (bool, error)
	SyncWorkContactTags(ctx context.Context, corpID int, groups []WorkContactTagSyncGroup) (WorkContactTagSyncResult, error)
	WorkContactByExternalUserID(ctx context.Context, externalUserID string) (WorkContactDetail, bool, error)
	SidebarWorkContactByExternalUserID(ctx context.Context, externalUserID string, corpID int, employeeID int) (WorkContactDetail, bool, error)
	ContactAccessibleToEmployee(ctx context.Context, contactID int, employeeID int, corpID int) (bool, error)
	WorkContactIndexPage(ctx context.Context, filter WorkContactIndexFilter) (WorkContactIndexPage, error)
	WorkContactLossPage(ctx context.Context, filter WorkContactLossFilter) (WorkContactLossPage, error)
	WorkContactShowByID(ctx context.Context, contactID int, employeeID int, corpID int) (WorkContactShow, bool, error)
	WorkContactRoomIndex(ctx context.Context, filter WorkContactRoomFilter) (WorkContactRoomPage, bool, error)
	WorkRoomOptions(ctx context.Context, filter WorkRoomOptionFilter) (WorkRoomOptionPage, error)
	WorkRoomIndexPage(ctx context.Context, filter WorkRoomIndexFilter) (WorkRoomIndexPage, error)
	WorkRoomExistsByCorpWXChatID(ctx context.Context, corpID int, wxChatID string) (bool, error)
	WorkRoomMemberStats(ctx context.Context, workRoomID int) ([]WorkRoomMemberStat, bool, error)
	WorkRoomGroupByID(ctx context.Context, groupID int) (WorkRoomGroupItem, bool, error)
	UpdateWorkRoomsGroup(ctx context.Context, values WorkRoomBatchUpdateValues) (int, error)
	SyncWorkRooms(ctx context.Context, corpID int, rooms []WorkRoomSyncRoom) (WorkRoomSyncResult, error)
	ContactEmployeeTracksByContactID(ctx context.Context, contactID int) ([]ContactEmployeeTrack, error)
	SidebarContactEmployeeTracksByContactID(ctx context.Context, contactID int, employeeID int, corpID int) ([]ContactEmployeeTrack, error)
	ContactProcessesByCorpID(ctx context.Context, corpID int) ([]ContactProcessStatus, error)
	CreateDefaultContactProcesses(ctx context.Context, corpID int) error
	ContactProcessByID(ctx context.Context, statusID int) (ContactProcessStatus, bool, error)
	UpdateContactProcessStatus(ctx context.Context, update ContactProcessStatusUpdate) error
	UpdateWorkContactProfile(ctx context.Context, values WorkContactUpdateValues) (WorkContactUpdateResult, bool, error)
	BatchLabelWorkContacts(ctx context.Context, contactIDs []int, tagIDs []int, employeeID int, corpID int) (int, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
}

type WorkReadHandler struct {
	store                   WorkReadStore
	cache                   LoginCache
	resolver                UserIDResolver
	sidebar                 UserIDResolver
	apiBaseURL              string
	authorizer              CorpAdminAuthorizer
	workContactUpdateClient WorkContactUpdateWeComClient
	workContactTagSync      WorkContactTagSyncClient
	workContactTagWrite     WorkContactTagWriteClient
	workContactSync         WorkContactSyncClient
	workRoomSync            WorkRoomSyncClient
	workEmployeeSync        WorkEmployeeSyncClient
	workEmployeePasswordKey string
}

func NewWorkReadHandler(store WorkReadStore, cache LoginCache, resolver UserIDResolver, apiBaseURL string) *WorkReadHandler {
	return &WorkReadHandler{
		store:      store,
		cache:      cache,
		resolver:   resolver,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
	}
}

func NewWorkReadHandlerWithAuthorizer(store WorkReadStore, cache LoginCache, resolver UserIDResolver, apiBaseURL string, authorizer CorpAdminAuthorizer) *WorkReadHandler {
	handler := NewWorkReadHandler(store, cache, resolver, apiBaseURL)
	handler.authorizer = authorizer
	return handler
}

func (h *WorkReadHandler) WithSidebarEmployeeResolver(resolver UserIDResolver) *WorkReadHandler {
	h.sidebar = resolver
	return h
}

func (h *WorkReadHandler) SearchCondition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	syncTime, err := h.store.WorkEmployeeSyncTime(r.Context(), principalScope.CorpIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"status": []map[string]any{
			{"id": 1, "name": "已激活"},
			{"id": 2, "name": "已禁用"},
			{"id": 4, "name": "未激活"},
			{"id": 5, "name": "退出企业"},
		},
		"contactAuth": []map[string]any{
			{"id": 1, "name": "是"},
			{"id": 2, "name": "否"},
		},
		"syncTime": syncTime,
	})
}

func (h *WorkReadHandler) DepartmentIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}

	search := r.URL.Query().Get("searchKeyWords")
	corpID := principalScope.CorpIDs[0]
	departments, err := h.store.WorkDepartmentsByCorp(r.Context(), corpID, search)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	employees, err := h.store.ActiveWorkEmployeesByCorp(r.Context(), corpID, search)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	for i := range employees {
		employees[i].EmployeeID = employees[i].ID
		employees[i].Avatar = h.fileFullURL(employees[i].Avatar)
	}

	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"department": departmentTree(departments),
		"employee":   employees,
	})
}

func (h *WorkReadHandler) MemberIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}

	rawDepartmentIDs := r.URL.Query().Get("departmentIds")
	if rawDepartmentIDs == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "部门id必传", nil)
		return
	}
	departmentIDs := parseIDList(rawDepartmentIDs)
	if len(departmentIDs) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []WorkDepartmentMember{})
		return
	}

	members, err := h.store.WorkDepartmentMembers(r.Context(), principalScope.CorpIDs[0], departmentIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", members)
}

func (h *WorkReadHandler) SelectByPhone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(principalScope.CorpIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}

	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "手机号码 必填", nil)
		return
	}
	if len(phone) != 11 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "手机号码 字符串长度为固定值：11", nil)
		return
	}
	rawType := strings.TrimSpace(r.URL.Query().Get("type"))
	if rawType != "" {
		typeValue, err := strconv.Atoi(rawType)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据类型 必需为整数", nil)
			return
		}
		if typeValue != 1 && typeValue != 2 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据类型 值必须在列表内：[1,2]", nil)
			return
		}
	}

	departments, err := h.store.WorkDepartmentsByEmployeeMobile(r.Context(), principalScope.CorpIDs[0], phone)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", departments)
}

func (h *WorkReadHandler) DepartmentPageIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未选择登录企业，不可操作", nil)
		return
	}

	departments, err := h.store.WorkDepartmentsByCorp(r.Context(), principalScope.CorpIDs[0], "")
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	filtered := filterWorkDepartmentPage(departments, r.URL.Query().Get("name"), r.URL.Query().Get("parentName"))
	page := workDepartmentPagePayload(filtered, positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 10))
	writeEnvelope(w, http.StatusOK, 200, "success", page)
}

func (h *WorkReadHandler) DepartmentShowEmployee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未选择登录企业，不可操作", nil)
		return
	}

	departmentID, err := queryIntRequired(r, "departmentId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "部门ID 必填", nil)
		return
	}
	if departmentID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "部门ID 不可小于1", nil)
		return
	}

	filter := WorkDepartmentEmployeeListFilter{
		CorpID:       principalScope.CorpIDs[0],
		DepartmentID: departmentID,
		Page:         positiveQueryInt(r, "page", 1),
		PerPage:      positiveQueryInt(r, "perPage", 10),
	}
	page, err := h.store.WorkDepartmentEmployeePage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, employee := range page.Items {
		list = append(list, map[string]any{
			"employeeId":   employee.EmployeeID,
			"employeeName": employee.EmployeeName,
			"phone":        employee.Phone,
			"roleName":     employee.RoleName,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   filter.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *WorkReadHandler) WorkEmployeeIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
		writeAccessError(w, err)
		return
	}

	rawCorpID := strings.TrimSpace(r.URL.Query().Get("corpId"))
	corpIDs := parseIDList(rawCorpID)
	if rawCorpID == "" {
		corpIDs = append([]int{}, principalScope.CorpIDs...)
	}
	if len(corpIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业微信不能为空", nil)
		return
	}
	status, ok := queryIntStrictDefault(r, "status", 0)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "成员状态必须为整数", nil)
		return
	}
	pageNumber, ok := positiveQueryIntStrict(r, "page", 1)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "页码必须为整数", nil)
		return
	}
	perPage, ok := positiveQueryIntStrict(r, "perPage", 10)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "页码必须为整数", nil)
		return
	}

	filter := WorkEmployeeIndexFilter{
		CorpIDs:     corpIDs,
		Name:        strings.TrimSpace(r.URL.Query().Get("name")),
		Status:      status,
		ContactAuth: strings.TrimSpace(r.URL.Query().Get("contactAuth")),
		Page:        pageNumber,
		PerPage:     perPage,
	}
	if filter.ContactAuth == "" {
		filter.ContactAuth = "all"
	}
	if dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context()); hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant {
		filter.RestrictEmployeeIDs = true
		filter.EmployeeIDs = append([]int{}, dashboardAccess.AllowedEmployeeIDs...)
	} else if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployeeIDs = true
		filter.EmployeeIDs = append([]int{}, access.DeptEmployeeIDs...)
	}

	page, err := h.store.WorkEmployeeIndexPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, employee := range page.Items {
		list = append(list, map[string]any{
			"id":                employee.ID,
			"name":              employee.Name,
			"thumbAvatar":       h.fileFullURL(employee.ThumbAvatar),
			"status":            employee.Status,
			"contactAuth":       employee.ContactAuth,
			"wxUserId":          employee.WXUserID,
			"corpId":            employee.CorpID,
			"gender":            genderName(employee.Gender),
			"messageNums":       employee.MessageNums,
			"sendMessageNums":   employee.SendMessageNums,
			"replyMessageRatio": employee.ReplyMessageRatio,
			"addNums":           employee.AddNums,
			"applyNums":         employee.ApplyNums,
			"invalidContact":    employee.InvalidContact,
			"averageReply":      employee.AverageReply,
			"statusName":        workEmployeeStatusName(employee.Status),
			"contactAuthName":   contactAuthName(employee.ContactAuth),
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"page":      filter.Page,
			"perPage":   filter.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *WorkReadHandler) ContactTagGroupIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}

	groups, err := h.store.WorkContactTagGroupsByCorp(r.Context(), principalScope.CorpIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(groups) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []map[string]any{})
		return
	}

	list := make([]map[string]any, 0, len(groups)+1)
	for _, group := range groups {
		list = append(list, map[string]any{
			"groupId":   group.ID,
			"groupName": group.GroupName,
		})
	}
	list = append(list, map[string]any{
		"groupId":   0,
		"groupName": "未分组",
	})
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) SidebarContactTagGroupIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}

	groups, err := h.store.WorkContactTagGroupsByCorp(r.Context(), []int{employee.CorpID})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(groups) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []map[string]any{})
		return
	}

	list := make([]map[string]any, 0, len(groups)+1)
	for _, group := range groups {
		list = append(list, map[string]any{
			"groupId":   group.ID,
			"groupName": group.GroupName,
		})
	}
	list = append(list, map[string]any{
		"groupId":   0,
		"groupName": "未分组",
	})
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) ContactTagGroupDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}

	rawGroupID := strings.TrimSpace(r.URL.Query().Get("groupId"))
	if rawGroupID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	groupID, err := strconv.Atoi(rawGroupID)
	if err != nil {
		groupID = 0
	}

	group, found, err := h.store.WorkContactTagGroupByID(r.Context(), groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []map[string]any{})
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":        group.ID,
		"groupName": group.GroupName,
	})
}

func (h *WorkReadHandler) ContactTagIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}

	filter := WorkContactTagFilter{
		CorpIDs: append([]int{}, principalScope.CorpIDs...),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 20),
	}
	if values, exists := r.URL.Query()["groupId"]; exists {
		rawGroupID := ""
		if len(values) > 0 {
			rawGroupID = strings.TrimSpace(values[0])
		}
		groupID, err := strconv.Atoi(rawGroupID)
		if err != nil {
			groupID = 0
		}
		filter.GroupID = &groupID
	}

	syncTime, err := h.store.WorkContactTagSyncTime(r.Context(), principalScope.CorpIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	page, err := h.store.WorkContactTagPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, tag := range page.Items {
		list = append(list, map[string]any{
			"id":         tag.ID,
			"name":       tag.Name,
			"contactNum": tag.ContactNum,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list":        list,
		"syncTagTime": syncTime,
	})
}

func (h *WorkReadHandler) ContactTagDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}

	rawTagID := strings.TrimSpace(r.URL.Query().Get("tagId"))
	if rawTagID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签id必传", nil)
		return
	}
	tagID, err := strconv.Atoi(rawTagID)
	if err != nil {
		tagID = 0
	}

	tag, found, err := h.store.WorkContactTagByID(r.Context(), tagID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []map[string]any{})
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"tagId":   tag.TagID,
		"tagName": tag.TagName,
		"groupId": tag.GroupID,
	})
}

func (h *WorkReadHandler) ContactTagList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}

	groups, err := h.store.WorkContactTagList(r.Context(), principalScope.CorpIDs, strings.TrimSpace(r.URL.Query().Get("name")))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		tags := make([]map[string]any, 0, len(group.Tags))
		for _, tag := range group.Tags {
			tags = append(tags, map[string]any{
				"id":                tag.ID,
				"wxContactTagId":    tag.WXContactTagID,
				"name":              tag.Name,
				"contactTagGroupId": tag.ContactTagGroupID,
			})
		}
		list = append(list, map[string]any{
			"id":        group.ID,
			"wxGroupId": group.WXGroupID,
			"groupName": group.GroupName,
			"tags":      tags,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) ContactTagAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}

	var groupID *int
	rawGroupID := strings.TrimSpace(r.URL.Query().Get("groupId"))
	if rawGroupID != "" {
		parsed, err := strconv.Atoi(rawGroupID)
		if err == nil {
			groupID = &parsed
		}
	}

	tags, err := h.store.WorkContactTags(r.Context(), principalScope.CorpIDs, groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		list = append(list, map[string]any{
			"id":   tag.ID,
			"name": tag.Name,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) SidebarContactTagAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}

	var groupID *int
	rawGroupID := strings.TrimSpace(r.URL.Query().Get("groupId"))
	if rawGroupID != "" {
		parsed, err := strconv.Atoi(rawGroupID)
		if err == nil {
			groupID = &parsed
		}
	}

	tags, err := h.store.WorkContactTags(r.Context(), []int{employee.CorpID}, groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		list = append(list, map[string]any{
			"id":   tag.ID,
			"name": tag.Name,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) SidebarWorkContactDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}

	externalUserID := strings.TrimSpace(r.URL.Query().Get("wxExternalUserid"))
	if externalUserID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "微信userId必须", nil)
		return
	}
	contact, found, err := h.store.SidebarWorkContactByExternalUserID(r.Context(), externalUserID, employee.CorpID, employee.ID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":     contact.ID,
		"name":   contact.Name,
		"avatar": h.fileFullURL(contact.Avatar),
		"corpId": contact.CorpID,
	})
}

func (h *WorkReadHandler) SidebarWorkContactShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	contactID, err := positiveQueryIntRequired(r, "contactId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	allowed, err := h.store.ContactAccessibleToEmployee(r.Context(), contactID, employee.ID, employee.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !allowed {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "无权访问该客户", nil)
		return
	}

	h.writeWorkContactShow(w, r, contactID, employee.ID, employee.CorpID)
}

func (h *WorkReadHandler) WorkContactShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	contactID, err := positiveQueryIntRequired(r, "contactId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	employeeID, err := positiveQueryIntRequired(r, "employeeId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "员工id必传", nil)
		return
	}
	if dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context()); hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant {
		allowed := false
		for _, id := range dashboardAccess.AllowedEmployeeIDs {
			if id == employeeID {
				allowed = true
				break
			}
		}
		if !allowed {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
			return
		}
	}

	h.writeWorkContactShow(w, r, contactID, employeeID, principalScope.Principal.CorpID)
}

func (h *WorkReadHandler) WorkContactIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
		writeAccessError(w, err)
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	filter := WorkContactIndexFilter{
		CorpID:            principalScope.CorpIDs[0],
		CurrentEmployeeID: principalScope.WorkEmployeeID,
		Remark:            strings.TrimSpace(r.URL.Query().Get("remark")),
		StartTime:         strings.TrimSpace(r.URL.Query().Get("startTime")),
		EndTime:           strings.TrimSpace(r.URL.Query().Get("endTime")),
		KeyWords:          strings.TrimSpace(r.URL.Query().Get("keyWords")),
		BusinessNo:        strings.TrimSpace(r.URL.Query().Get("businessNo")),
		FieldValue:        strings.TrimSpace(r.URL.Query().Get("fieldValue")),
		RoomIDs:           parseIDList(r.URL.Query().Get("roomId")),
		Page:              positiveQueryInt(r, "page", 1),
		PerPage:           positiveQueryInt(r, "perPage", 20),
	}
	if dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context()); hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant {
		filter.RestrictEmployees = true
		filter.EmployeeIDs = append([]int{}, dashboardAccess.AllowedEmployeeIDs...)
	} else if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployees = true
		filter.EmployeeIDs = append([]int{}, access.DeptEmployeeIDs...)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("employeeId")); raw != "" {
		if raw == "空" {
			filter.RestrictEmployees = true
			filter.EmployeeIDs = []int{}
		} else {
			employeeIDs := parseIDList(raw)
			if filter.RestrictEmployees {
				filter.EmployeeIDs = intersectInts(filter.EmployeeIDs, employeeIDs)
			} else {
				filter.RestrictEmployees = true
				filter.EmployeeIDs = employeeIDs
			}
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("addWay")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			filter.AddWay = &value
		}
	}
	if values, exists := r.URL.Query()["gender"]; exists {
		raw := ""
		if len(values) > 0 {
			raw = strings.TrimSpace(values[0])
		}
		if value, err := strconv.Atoi(raw); err == nil && value != 3 {
			filter.Gender = &value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("fieldId")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value != 0 {
			filter.FieldID = &value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("groupNum")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value != 3 {
			filter.GroupNum = &value
		}
	}

	page, err := h.store.WorkContactIndexPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if page.EmptyData {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	userPayload := workContactIndexUserPayload(user, principalScope, access)
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":           item.ID,
			"employeeId":   item.EmployeeID,
			"contactId":    item.ContactID,
			"remark":       item.Remark,
			"createTime":   item.CreateTime,
			"addWay":       item.AddWay,
			"addWayText":   workContactAddWayText(item.AddWay),
			"genderText":   workContactGenderText(item.Gender),
			"businessNo":   item.BusinessNo,
			"name":         item.Name,
			"avatar":       h.fileFullURL(item.Avatar),
			"gender":       item.Gender,
			"roomName":     item.RoomName,
			"employeeName": item.EmployeeName,
			"tag":          item.Tag,
			"isContact":    item.IsContact,
			"user":         userPayload,
		})
	}
	perPage := page.PerPage
	if page.FilterNoData {
		perPage = 20
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   perPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list":            list,
		"syncContactTime": page.SyncContactTime,
	})
}

func (h *WorkReadHandler) WorkContactLoss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
		writeAccessError(w, err)
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	filter := WorkContactLossFilter{
		CorpID:  principalScope.CorpIDs[0],
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 20),
	}
	if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployees = true
		filter.EmployeeIDs = append([]int{}, access.DeptEmployeeIDs...)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("employeeId")); raw != "" {
		employeeIDs := parseIDList(raw)
		if filter.RestrictEmployees {
			filter.EmployeeIDs = intersectInts(filter.EmployeeIDs, employeeIDs)
		} else {
			filter.RestrictEmployees = true
			filter.EmployeeIDs = employeeIDs
		}
	}

	page, err := h.store.WorkContactLossPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	userPayload := workContactIndexUserPayload(user, principalScope, access)
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":           item.ID,
			"employeeId":   item.EmployeeID,
			"contactId":    item.ContactID,
			"deletedAt":    item.DeletedAt,
			"avatar":       h.fileFullURL(item.Avatar),
			"name":         item.Name,
			"tag":          item.Tag,
			"employeeName": item.EmployeeName,
			"remark":       item.Remark,
			"user":         userPayload,
		})
	}
	perPage := page.PerPage
	if page.NoData {
		perPage = 20
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   perPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *WorkReadHandler) WorkContactRoomIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	workRoomID, err := positiveQueryIntRequired(r, "workRoomId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群ID 必填", nil)
		return
	}
	var status *int
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "status must be integer", nil)
			return
		}
		if value != 0 {
			status = &value
		}
	}
	filter := WorkContactRoomFilter{
		WorkRoomID: workRoomID,
		CorpID:     corpID,
		Status:     status,
		Name:       strings.TrimSpace(r.URL.Query().Get("name")),
		StartTime:  strings.TrimSpace(r.URL.Query().Get("startTime")),
		EndTime:    strings.TrimSpace(r.URL.Query().Get("endTime")),
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 10),
	}
	page, found, err := h.store.WorkContactRoomIndex(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前客户群不存在", nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"workContactRoomId": item.WorkContactRoomID,
			"name":              item.Name,
			"avatar":            h.fileFullURL(item.Avatar),
			"isOwner":           item.IsOwner,
			"joinTime":          item.JoinTime,
			"outRoomTime":       item.OutRoomTime,
			"otherRooms":        item.OtherRooms,
			"joinScene":         item.JoinScene,
			"joinSceneText":     workContactRoomJoinSceneText(item.JoinScene),
			"type":              item.Type,
			"contactId":         item.ContactID,
			"employeeId":        item.EmployeeID,
			"contactEmployeeId": item.ContactEmployeeID,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"memberNum":  page.MemberNum,
		"outRoomNum": page.OutRoomNum,
		"page": map[string]any{
			"perPage":   strconv.Itoa(page.PerPage),
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *WorkReadHandler) WorkRoomRoomIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	var roomGroupID *int
	if raw := strings.TrimSpace(r.URL.Query().Get("roomGroupId")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			roomGroupID = &value
		}
	}
	page, err := h.store.WorkRoomOptions(r.Context(), WorkRoomOptionFilter{
		CorpIDs:     append([]int{}, principalScope.CorpIDs...),
		Name:        strings.TrimSpace(r.URL.Query().Get("name")),
		RoomGroupID: roomGroupID,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"roomMax":    item.RoomMax,
			"roomId":     item.RoomID,
			"roomName":   item.RoomName,
			"currentNum": item.CurrentNum,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"total": page.Total,
		"list":  list,
	})
}

func (h *WorkReadHandler) WorkRoomIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	ownerIDs := parseIDList(r.URL.Query().Get("workRoomOwnerId"))
	filter := WorkRoomIndexFilter{
		CorpID:        corpID,
		OwnerIDs:      ownerIDs,
		RestrictOwner: len(ownerIDs) > 0,
		Name:          strings.TrimSpace(r.URL.Query().Get("workRoomName")),
		StartTime:     strings.TrimSpace(r.URL.Query().Get("startTime")),
		EndTime:       strings.TrimSpace(r.URL.Query().Get("endTime")),
		Page:          positiveQueryInt(r, "page", 1),
		PerPage:       positiveQueryInt(r, "perPage", 10),
	}
	if dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context()); hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant {
		filter.RestrictOwner = true
		if len(filter.OwnerIDs) == 0 {
			filter.OwnerIDs = append([]int{}, dashboardAccess.AllowedEmployeeIDs...)
		} else {
			filter.OwnerIDs = intersectInts(filter.OwnerIDs, dashboardAccess.AllowedEmployeeIDs)
		}
	} else if access.DataPermission != DataPermissionAll {
		filter.RestrictOwner = true
		if len(filter.OwnerIDs) == 0 {
			filter.OwnerIDs = append([]int{}, access.DeptEmployeeIDs...)
		} else {
			filter.OwnerIDs = intersectInts(filter.OwnerIDs, access.DeptEmployeeIDs)
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("roomGroupId")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			filter.RoomGroupID = &value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("workRoomStatus")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			filter.Status = &value
		}
	}

	page, err := h.store.WorkRoomIndexPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	memberTotal := 0
	activeTotal := 0
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		memberTotal += item.MemberNum
		if item.Status == 1 {
			activeTotal++
		}
		list = append(list, map[string]any{
			"workRoomId":  item.WorkRoomID,
			"memberNum":   item.MemberNum,
			"memberCount": item.MemberNum,
			"roomName":    item.RoomName,
			"ownerId":     item.OwnerID,
			"ownerName":   item.OwnerName,
			"roomGroup":   item.RoomGroup,
			"status":      item.Status,
			"statusText":  workRoomStatusText(item.Status),
			"activeStatus": func() string {
				if item.Status == 1 {
					return "active"
				}
				return "inactive"
			}(),
			"inRoomNum":  item.InRoomNum,
			"outRoomNum": item.OutRoomNum,
			"notice":     item.Notice,
			"createTime": item.CreateTime,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"memberTotal": memberTotal,
		"activeTotal": activeTotal,
		"page": map[string]any{
			"perPage":   strconv.Itoa(page.PerPage),
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *WorkReadHandler) SidebarWorkRoomManage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if roomID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "roomId 必传", nil)
		return
	}
	found, err := h.store.WorkRoomExistsByCorpWXChatID(r.Context(), employee.CorpID, roomID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) WorkRoomStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	req, ok := parseWorkRoomStatisticRequest(w, r, false)
	if !ok {
		return
	}
	members, found, err := h.store.WorkRoomMemberStats(r.Context(), req.WorkRoomID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该客户群数据错误", nil)
		return
	}
	data := h.workRoomStatisticSummary(req, members)
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *WorkReadHandler) WorkRoomStatisticsIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	req, ok := parseWorkRoomStatisticRequest(w, r, true)
	if !ok {
		return
	}
	members, found, err := h.store.WorkRoomMemberStats(r.Context(), req.WorkRoomID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该客户群数据错误", nil)
		return
	}
	points := h.workRoomStatisticIndexPoints(req, members)
	total := len(points)
	totalPage := 0
	if req.PerPage > 0 {
		totalPage = (total + req.PerPage - 1) / req.PerPage
	}
	start := (req.Page - 1) * req.PerPage
	if start > total {
		start = total
	}
	end := start + req.PerPage
	if end > total {
		end = total
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   strconv.Itoa(req.PerPage),
			"total":     total,
			"totalPage": totalPage,
		},
		"list": workRoomStatisticPointPayloads(points[start:end], true),
	})
}

func (h *WorkReadHandler) writeWorkContactShow(w http.ResponseWriter, r *http.Request, contactID int, employeeID int, corpID int) {
	info, _, err := h.store.WorkContactShowByID(r.Context(), contactID, employeeID, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	tags := make([]map[string]any, 0, len(info.Tags))
	for _, tag := range info.Tags {
		tags = append(tags, map[string]any{
			"tagId":   tag.TagID,
			"tagName": tag.TagName,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"name":         info.Name,
		"avatar":       h.fileFullURL(info.Avatar),
		"gender":       info.Gender,
		"genderText":   workContactGenderText(info.Gender),
		"businessNo":   info.BusinessNo,
		"remark":       info.Remark,
		"description":  info.Description,
		"tag":          tags,
		"roomName":     info.RoomNames,
		"employeeName": info.EmployeeName,
	})
}

func (h *WorkReadHandler) SidebarWorkContactTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	contactID, ok := contactEmployeeTrackID(w, r)
	if !ok {
		return
	}
	allowed, err := h.store.ContactAccessibleToEmployee(r.Context(), contactID, employee.ID, employee.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !allowed {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "无权访问该客户", nil)
		return
	}
	h.writeSidebarContactEmployeeTrackByID(w, r, contactID, employee)
}

func (h *WorkReadHandler) WorkContactTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}

	h.writeContactEmployeeTrack(w, r)
}

func (h *WorkReadHandler) WorkContactSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}

	list := make([]map[string]any, 0, len(workContactSourceOptions))
	for _, option := range workContactSourceOptions {
		list = append(list, map[string]any{
			"addWay":     option.AddWay,
			"addWayText": option.AddWayText,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) SidebarContactProcessStatusIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}

	statuses, err := h.store.ContactProcessesByCorpID(r.Context(), employee.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(statuses) == 0 {
		if err := h.store.CreateDefaultContactProcesses(r.Context(), employee.CorpID); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		statuses, err = h.store.ContactProcessesByCorpID(r.Context(), employee.CorpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}

	list := make([]map[string]any, 0, len(statuses))
	for _, status := range statuses {
		list = append(list, map[string]any{
			"id":   status.ID,
			"name": status.Name,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) writeContactEmployeeTrack(w http.ResponseWriter, r *http.Request) {
	contactID, ok := contactEmployeeTrackID(w, r)
	if !ok {
		return
	}
	h.writeContactEmployeeTrackByID(w, r, contactID)
}

func contactEmployeeTrackID(w http.ResponseWriter, r *http.Request) (int, bool) {
	rawContactID := strings.TrimSpace(r.URL.Query().Get("contactId"))
	if rawContactID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return 0, false
	}
	contactID, err := strconv.Atoi(rawContactID)
	if err != nil || contactID < 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户ID 不可小于1", nil)
		return 0, false
	}
	return contactID, true
}

func (h *WorkReadHandler) writeContactEmployeeTrackByID(w http.ResponseWriter, r *http.Request, contactID int) {
	tracks, err := h.store.ContactEmployeeTracksByContactID(r.Context(), contactID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	h.writeContactEmployeeTrackPayload(w, tracks)
}

func (h *WorkReadHandler) writeSidebarContactEmployeeTrackByID(w http.ResponseWriter, r *http.Request, contactID int, employee SidebarEmployee) {
	tracks, err := h.store.SidebarContactEmployeeTracksByContactID(r.Context(), contactID, employee.ID, employee.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	h.writeContactEmployeeTrackPayload(w, tracks)
}

func (h *WorkReadHandler) writeContactEmployeeTrackPayload(w http.ResponseWriter, tracks []ContactEmployeeTrack) {
	list := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		list = append(list, map[string]any{
			"id":        track.ID,
			"content":   track.Content,
			"createdAt": track.CreatedAt,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkReadHandler) SidebarContactProcessStatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	req, err := parseContactProcessStatusUpdateRequest(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if req.ContactID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	if req.StatusID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "跟进状态id必传", nil)
		return
	}

	process, _, err := h.store.ContactProcessByID(r.Context(), req.StatusID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	update := ContactProcessStatusUpdate{
		ContactID:  req.ContactID,
		StatusID:   req.StatusID,
		EmployeeID: employee.ID,
		CorpID:     employee.CorpID,
		Event:      contactEmployeeTrackEventProcessStatus,
		Content:    "编辑用户跟进状态：" + process.Name,
	}
	if err := h.store.UpdateContactProcessStatus(r.Context(), update); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

var workContactSourceOptions = []workContactSourceOption{
	{AddWay: 0, AddWayText: "其他渠道"},
	{AddWay: 1, AddWayText: "扫描二维码"},
	{AddWay: 1001, AddWayText: "渠道活码"},
	{AddWay: 1002, AddWayText: "自动拉群"},
	{AddWay: 1003, AddWayText: "裂变引流"},
	{AddWay: 2, AddWayText: "搜索手机号"},
	{AddWay: 3, AddWayText: "名片分享"},
	{AddWay: 4, AddWayText: "群聊"},
	{AddWay: 5, AddWayText: "手机通讯录"},
	{AddWay: 6, AddWayText: "微信联系人"},
	{AddWay: 7, AddWayText: "来自微信的添加好友申请"},
	{AddWay: 8, AddWayText: "安装第三方应用时自动添加的客服人员"},
	{AddWay: 9, AddWayText: "搜索邮箱"},
	{AddWay: 201, AddWayText: "内部成员共享"},
	{AddWay: 202, AddWayText: "管理员/负责人分配"},
}

func workContactAddWayText(addWay int) string {
	for _, option := range workContactSourceOptions {
		if option.AddWay == addWay {
			return option.AddWayText
		}
	}
	return ""
}

func workContactIndexUserPayload(user User, principalScope DashboardRequestScope, access AccessContext) map[string]any {
	return map[string]any{
		"id":              user.ID,
		"phone":           user.Phone,
		"name":            user.Name,
		"gender":          user.Gender,
		"department":      user.Department,
		"position":        user.Position,
		"loginTime":       user.LoginTime,
		"status":          user.Status,
		"corpIds":         append([]int{}, principalScope.CorpIDs...),
		"workEmployeeId":  principalScope.WorkEmployeeID,
		"requestSource":   principalScope.RequestSource,
		"tenantId":        user.TenantID,
		"roleId":          access.RoleID,
		"isSuperAdmin":    user.IsSuperAdmin,
		"dataPermission":  access.DataPermission,
		"deptEmployeeIds": append([]int{}, access.DeptEmployeeIDs...),
	}
}

func (h *WorkReadHandler) ContactTagGroupStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupName := stringParam(params, "groupName")
	if groupName == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称必传", nil)
		return
	}
	exists, err := h.store.WorkContactTagGroupNameExists(r.Context(), corpID, groupName, 0)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "已存在相同分组名", nil)
		return
	}
	if _, err := h.store.CreateWorkContactTagGroup(r.Context(), WorkContactTagGroupWrite{CorpID: corpID, GroupName: groupName}); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户标签分组创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) ContactTagGroupUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, okInt, err := intParam(params, "groupId")
	if err != nil || !okInt || groupID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	groupName := stringParam(params, "groupName")
	if groupName == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称必传", nil)
		return
	}
	isUpdate, okInt, err := intParam(params, "isUpdate")
	if err != nil || !okInt {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "是否修改信息必传", nil)
		return
	}
	if isUpdate == 1 {
		exists, err := h.store.WorkContactTagGroupNameExists(r.Context(), corpID, groupName, groupID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if exists {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "已存在相同分组名", nil)
			return
		}
	}
	updated, err := h.store.UpdateWorkContactTagGroup(r.Context(), corpID, groupID, groupName)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户标签分组编辑失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) ContactTagGroupDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, okInt, err := intParam(params, "groupId")
	if err != nil || !okInt || groupID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	group, found, err := h.store.WorkContactTagGroupByID(r.Context(), groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "查询不到该标签分组信息", nil)
		return
	}
	deleted, err := h.store.DeleteWorkContactTagGroupCascade(r.Context(), corpID, groupID)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	if err := h.syncRemoteContactTagGroupDelete(r.Context(), corpID, group); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) ContactTagStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, tagNames, ok := parseContactTagWriteParams(w, params)
	if !ok {
		return
	}
	exists, err := h.store.WorkContactTagNamesExist(r.Context(), corpID, groupID, tagNames, nil)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该分组下已存在相同标签名", nil)
		return
	}
	if err := h.store.CreateWorkContactTags(r.Context(), WorkContactTagWrite{CorpID: corpID, GroupID: groupID, TagNames: tagNames}); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户标签创建失败", nil)
		return
	}
	if err := h.addRemoteContactTags(r.Context(), corpID, groupID, tagNames); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) ContactTagUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	tagID, okInt, err := intParam(params, "tagId")
	if err != nil || !okInt || tagID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签id必传", nil)
		return
	}
	groupID, tagNames, ok := parseContactTagWriteParams(w, params)
	if !ok {
		return
	}
	isUpdate, okInt, err := intParam(params, "isUpdate")
	if err != nil || !okInt {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "是否修改信息必传", nil)
		return
	}
	if isUpdate == 1 {
		exists, err := h.store.WorkContactTagNamesExist(r.Context(), corpID, groupID, []string{tagNames[0]}, []int{tagID})
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if exists {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该分组下已存在相同标签名", nil)
			return
		}
	}
	current, found, err := h.store.WorkContactTagByID(r.Context(), tagID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "查询不到该标签信息", nil)
		return
	}
	updated, err := h.store.UpdateWorkContactTag(r.Context(), corpID, tagID, groupID, tagNames[0])
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户标签编辑失败", nil)
		return
	}
	if err := h.syncRemoteContactTagUpdate(r.Context(), corpID, current, groupID, tagNames[0]); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) ContactTagDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	tagIDs, err := intSliceParam(params, "tagId")
	if err != nil || len(tagIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签id必传", nil)
		return
	}
	tags, err := h.tagsByIDs(r.Context(), tagIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(tags) == 0 {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "查询不到该标签信息", nil)
		return
	}
	deleted, err := h.store.DeleteWorkContactTags(r.Context(), corpID, tagIDs)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "标签删除失败", nil)
		return
	}
	if err := h.syncRemoteContactTagDelete(r.Context(), corpID, tags); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) ContactTagMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	tagIDs, err := intSliceParam(params, "tagId")
	if err != nil || len(tagIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签id必传", nil)
		return
	}
	groupID, okInt, err := intParam(params, "groupId")
	if err != nil || !okInt || groupID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	tags, err := h.tagsByIDs(r.Context(), tagIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(tags) == 0 {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "查询不到标签信息", nil)
		return
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.TagName)
	}
	exists, err := h.store.WorkContactTagNamesExist(r.Context(), corpID, groupID, names, tagIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该分组下已有相同标签", nil)
		return
	}
	moved, err := h.store.MoveWorkContactTags(r.Context(), corpID, tagIDs, groupID)
	if err != nil || !moved {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "移动标签失败", nil)
		return
	}
	if err := h.syncRemoteContactTagMove(r.Context(), corpID, tags, groupID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func parseContactTagWriteParams(w http.ResponseWriter, params map[string]any) (int, []string, bool) {
	groupID, okInt, err := intParam(params, "groupId")
	if err != nil || !okInt || groupID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return 0, nil, false
	}
	tagNames := stringSliceParam(params, "tagName")
	if len(tagNames) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签名称必传", nil)
		return 0, nil, false
	}
	return groupID, tagNames, true
}

func (h *WorkReadHandler) addRemoteContactTags(ctx context.Context, corpID int, groupID int, tagNames []string) error {
	if h.workContactTagWrite == nil || groupID == 0 {
		return nil
	}
	group, found, err := h.store.WorkContactTagGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("查询不到该标签分组信息")
	}
	credential, ok, err := h.workContactTagCredential(ctx, corpID)
	if err != nil || !ok {
		return err
	}
	remoteGroup, err := h.workContactTagWrite.AddCorpTags(ctx, credential, WorkContactTagAddRequest{
		WXGroupID: group.WXGroupID,
		GroupName: group.GroupName,
		TagNames:  tagNames,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(group.WXGroupID) == "" && strings.TrimSpace(remoteGroup.WXGroupID) != "" {
		if _, err := h.store.UpdateWorkContactTagGroupWXID(ctx, corpID, groupID, remoteGroup.WXGroupID); err != nil {
			return err
		}
	}
	tagWXIDs := map[string]string{}
	for _, tag := range remoteGroup.Tags {
		name := strings.TrimSpace(tag.Name)
		wxTagID := strings.TrimSpace(tag.WXContactTagID)
		if name != "" && wxTagID != "" {
			tagWXIDs[name] = wxTagID
		}
	}
	if len(tagWXIDs) > 0 {
		if err := h.store.UpdateWorkContactTagWXIDsByName(ctx, corpID, groupID, tagWXIDs); err != nil {
			return err
		}
	}
	return nil
}

func (h *WorkReadHandler) syncRemoteContactTagUpdate(ctx context.Context, corpID int, current WorkContactTagDetail, targetGroupID int, tagName string) error {
	if h.workContactTagWrite == nil {
		return nil
	}
	credential, ok, err := h.workContactTagCredential(ctx, corpID)
	if err != nil || !ok {
		return err
	}
	if targetGroupID == current.GroupID {
		if targetGroupID == 0 {
			return nil
		}
		if strings.TrimSpace(current.WXContactTagID) != "" {
			return h.workContactTagWrite.UpdateCorpTag(ctx, credential, current.WXContactTagID, tagName)
		}
		return h.addRemoteContactTags(ctx, corpID, targetGroupID, []string{tagName})
	}
	if strings.TrimSpace(current.WXContactTagID) != "" {
		if err := h.workContactTagWrite.DeleteCorpTags(ctx, credential, []string{current.WXContactTagID}, nil); err != nil {
			return err
		}
	}
	if targetGroupID != 0 {
		return h.addRemoteContactTags(ctx, corpID, targetGroupID, []string{tagName})
	}
	return nil
}

func (h *WorkReadHandler) syncRemoteContactTagDelete(ctx context.Context, corpID int, tags []WorkContactTagDetail) error {
	if h.workContactTagWrite == nil {
		return nil
	}
	wxTagIDs := make([]string, 0, len(tags))
	for _, tag := range tags {
		if wxTagID := strings.TrimSpace(tag.WXContactTagID); wxTagID != "" {
			wxTagIDs = append(wxTagIDs, wxTagID)
		}
	}
	if len(wxTagIDs) == 0 {
		return nil
	}
	credential, ok, err := h.workContactTagCredential(ctx, corpID)
	if err != nil || !ok {
		return err
	}
	return h.workContactTagWrite.DeleteCorpTags(ctx, credential, wxTagIDs, nil)
}

func (h *WorkReadHandler) syncRemoteContactTagMove(ctx context.Context, corpID int, tags []WorkContactTagDetail, targetGroupID int) error {
	if h.workContactTagWrite == nil {
		return nil
	}
	if err := h.syncRemoteContactTagDelete(ctx, corpID, tags); err != nil {
		return err
	}
	if targetGroupID == 0 {
		return nil
	}
	tagNames := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.TagName); name != "" {
			tagNames = append(tagNames, name)
		}
	}
	return h.addRemoteContactTags(ctx, corpID, targetGroupID, tagNames)
}

func (h *WorkReadHandler) syncRemoteContactTagGroupDelete(ctx context.Context, corpID int, group WorkContactTagGroup) error {
	if h.workContactTagWrite == nil || strings.TrimSpace(group.WXGroupID) == "" {
		return nil
	}
	credential, ok, err := h.workContactTagCredential(ctx, corpID)
	if err != nil || !ok {
		return err
	}
	return h.workContactTagWrite.DeleteCorpTags(ctx, credential, nil, []string{group.WXGroupID})
}

func (h *WorkReadHandler) workContactTagCredential(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return RoomWelcomeCorpCredential{}, false, err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return RoomWelcomeCorpCredential{}, false, fmt.Errorf("企业授权信息错误")
	}
	return credential, true, nil
}

func (h *WorkReadHandler) tagsByIDs(ctx context.Context, tagIDs []int) ([]WorkContactTagDetail, error) {
	tags := make([]WorkContactTagDetail, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		tag, found, err := h.store.WorkContactTagByID(ctx, tagID)
		if err != nil {
			return nil, err
		}
		if found {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (h *WorkReadHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, principalScope, true
}

func (h *WorkReadHandler) resolveSidebarAccess(w http.ResponseWriter, r *http.Request) (SidebarEmployee, bool) {
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

func (h *WorkReadHandler) authorize(ctx context.Context, r *http.Request, userID int, principalScope DashboardRequestScope) error {
	_, err := h.authorizeAccess(ctx, r, userID, principalScope)
	return err
}

func (h *WorkReadHandler) authorizeAccess(ctx context.Context, r *http.Request, userID int, principalScope DashboardRequestScope) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{DataPermission: DataPermissionAll}, nil
	}
	corpID := 0
	if len(principalScope.CorpIDs) > 0 {
		corpID = principalScope.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID)
}

func (h *WorkReadHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func parseWorkRoomStatisticRequest(w http.ResponseWriter, r *http.Request, withPage bool) (WorkRoomStatisticRequest, bool) {
	workRoomID, okInt, err := intParam(map[string]any{"workRoomId": r.URL.Query().Get("workRoomId")}, "workRoomId")
	if err != nil || !okInt || workRoomID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群ID 必填", nil)
		return WorkRoomStatisticRequest{}, false
	}
	statType, okInt, err := intParam(map[string]any{"type": r.URL.Query().Get("type")}, "type")
	if err != nil || !okInt {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "统计类型 必填", nil)
		return WorkRoomStatisticRequest{}, false
	}
	if statType < 1 || statType > 3 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "统计类型 值必须在列表内：[1,2,3]", nil)
		return WorkRoomStatisticRequest{}, false
	}
	req := WorkRoomStatisticRequest{
		WorkRoomID: workRoomID,
		Type:       statType,
		StartTime:  strings.TrimSpace(r.URL.Query().Get("startTime")),
		EndTime:    strings.TrimSpace(r.URL.Query().Get("endTime")),
		Page:       1,
		PerPage:    10,
	}
	if req.Type == 1 {
		if req.StartTime == "" || req.EndTime == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "按天统计开始和结束时间必传", nil)
			return WorkRoomStatisticRequest{}, false
		}
		if _, ok := parseWorkRoomDate(req.StartTime); !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "开始时间格式错误", nil)
			return WorkRoomStatisticRequest{}, false
		}
		if _, ok := parseWorkRoomDate(req.EndTime); !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "结束时间格式错误", nil)
			return WorkRoomStatisticRequest{}, false
		}
	}
	if withPage {
		req.Page = positiveQueryInt(r, "page", 1)
		req.PerPage = positiveQueryInt(r, "perPage", 10)
	}
	return req, true
}

func (h *WorkReadHandler) workRoomStatisticSummary(req WorkRoomStatisticRequest, members []WorkRoomMemberStat) map[string]any {
	points := workRoomStatisticBasePoints(req, false)
	index := workRoomStatisticPointIndex(points)
	data := map[string]any{
		"addNum":      0,
		"outNum":      0,
		"total":       0,
		"outTotal":    0,
		"addNumRange": 0,
		"outNumRange": 0,
		"list":        []map[string]any{},
	}
	today := time.Now().Format("2006-01-02")
	for _, member := range members {
		joinDay := workRoomDateKey(member.JoinTime, 1)
		joinKey := workRoomDateKey(member.JoinTime, req.Type)
		outKey := workRoomDateKey(member.OutTime, req.Type)
		if member.Status == 1 {
			data["total"] = data["total"].(int) + 1
		}
		if joinDay == today {
			data["addNum"] = data["addNum"].(int) + 1
		}
		if pointIndex, ok := index[joinKey]; ok {
			points[pointIndex].AddNum++
			data["addNumRange"] = data["addNumRange"].(int) + 1
		}
		if member.Status == 2 {
			if workRoomDateKey(member.OutTime, 1) == today {
				data["outNum"] = data["outNum"].(int) + 1
			}
			data["outTotal"] = data["outTotal"].(int) + 1
			if pointIndex, ok := index[outKey]; ok {
				points[pointIndex].OutNum++
				data["outNumRange"] = data["outNumRange"].(int) + 1
			}
		}
	}
	data["list"] = workRoomStatisticPointPayloads(points, false)
	return data
}

func (h *WorkReadHandler) workRoomStatisticIndexPoints(req WorkRoomStatisticRequest, members []WorkRoomMemberStat) []WorkRoomStatisticPoint {
	points := workRoomStatisticBasePoints(req, true)
	index := workRoomStatisticPointIndex(points)
	for _, member := range members {
		joinKey := workRoomDateKey(member.JoinTime, req.Type)
		outKey := workRoomDateKey(member.OutTime, req.Type)
		if pointIndex, ok := index[joinKey]; ok {
			points[pointIndex].AddNum++
		}
		if pointIndex, ok := index[outKey]; ok {
			points[pointIndex].OutNum++
		}
		for pointIndex := range points {
			if joinKey != "" && joinKey <= points[pointIndex].Time {
				points[pointIndex].Total++
			}
			if member.Status == 2 && outKey != "" && outKey <= points[pointIndex].Time {
				points[pointIndex].OutTotal++
			}
		}
	}
	return points
}

func workRoomStatisticBasePoints(req WorkRoomStatisticRequest, includeTotals bool) []WorkRoomStatisticPoint {
	points := make([]WorkRoomStatisticPoint, 0)
	switch req.Type {
	case 1:
		start, okStart := parseWorkRoomDate(req.StartTime)
		end, okEnd := parseWorkRoomDate(req.EndTime)
		if !okStart || !okEnd || end.Before(start) {
			return points
		}
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			points = append(points, WorkRoomStatisticPoint{Time: day.Format("2006-01-02")})
		}
	case 2:
		start := time.Now().AddDate(0, 0, -6)
		for i := 0; i < 7; i++ {
			points = append(points, WorkRoomStatisticPoint{Time: start.AddDate(0, 0, i).Format("2006-01-02")})
		}
	case 3:
		start := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local).AddDate(-1, 1, 0)
		for i := 0; i < 12; i++ {
			points = append(points, WorkRoomStatisticPoint{Time: start.AddDate(0, i, 0).Format("2006-01")})
		}
	}
	if !includeTotals {
		for index := range points {
			points[index].Total = 0
			points[index].OutTotal = 0
		}
	}
	return points
}

func workRoomStatisticPointIndex(points []WorkRoomStatisticPoint) map[string]int {
	index := make(map[string]int, len(points))
	for i, point := range points {
		index[point.Time] = i
	}
	return index
}

func workRoomStatisticPointPayloads(points []WorkRoomStatisticPoint, includeTotals bool) []map[string]any {
	list := make([]map[string]any, 0, len(points))
	for _, point := range points {
		item := map[string]any{
			"time":   point.Time,
			"addNum": point.AddNum,
			"outNum": point.OutNum,
		}
		if includeTotals {
			item["total"] = point.Total
			item["outTotal"] = point.OutTotal
		}
		list = append(list, item)
	}
	return list
}

func workRoomDateKey(raw string, statType int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) >= len("2006-01-02") {
		if statType == 3 && len(raw) >= len("2006-01") {
			return raw[:len("2006-01")]
		}
		return raw[:len("2006-01-02")]
	}
	return raw
}

func parseWorkRoomDate(raw string) (time.Time, bool) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), time.Local)
	return parsed, err == nil
}

func departmentTree(departments []WorkDepartment) []*WorkDepartment {
	items := make(map[int]*WorkDepartment, len(departments))
	ordered := make([]*WorkDepartment, 0, len(departments))
	for _, department := range departments {
		department.Son = nil
		current := department
		items[current.ID] = &current
		ordered = append(ordered, &current)
	}

	tree := make([]*WorkDepartment, 0, len(ordered))
	for _, item := range ordered {
		if parent, ok := items[item.ParentID]; ok {
			parent.Son = append(parent.Son, item)
			continue
		}
		tree = append(tree, item)
	}
	return tree
}

func parseIDList(raw string) []int {
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func intersectInts(values []int, allowed []int) []int {
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, value := range allowed {
		if value > 0 {
			allowedSet[value] = struct{}{}
		}
	}
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := allowedSet[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func workRoomStatusText(status int) string {
	names := map[int]string{
		0: "正常",
		1: "跟进人离职",
		2: "离职继承中",
		3: "离职继承完成",
	}
	return names[status]
}

type contactProcessStatusUpdateRequest struct {
	ContactID int `json:"contactId"`
	StatusID  int `json:"statusId"`
}

func parseContactProcessStatusUpdateRequest(r *http.Request) (contactProcessStatusUpdateRequest, error) {
	var req contactProcessStatusUpdateRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		req.ContactID, _ = strconv.Atoi(r.FormValue("contactId"))
		req.StatusID, _ = strconv.Atoi(r.FormValue("statusId"))
		return req, nil
	}

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func workContactGenderText(gender int) string {
	switch gender {
	case 1:
		return "男"
	case 2:
		return "女"
	default:
		return "未知"
	}
}

func workContactRoomJoinSceneText(scene int) string {
	switch scene {
	case 1:
		return "由成员邀请入群（直接邀请入群）"
	case 2:
		return "由成员邀请入群（通过邀请链接入群）"
	case 3:
		return "通过扫描群二维码入群"
	default:
		return ""
	}
}

func filterWorkDepartmentPage(departments []WorkDepartment, name string, parentName string) []WorkDepartment {
	name = strings.TrimSpace(name)
	parentName = strings.TrimSpace(parentName)
	sort.SliceStable(departments, func(i, j int) bool {
		return departments[i].ID > departments[j].ID
	})
	if name == "" && parentName == "" {
		return departments
	}

	var idSet map[int]struct{}
	if name != "" {
		idSet = departmentDescendantIDsByName(departments, name)
	}
	if parentName != "" {
		parentSet := departmentDescendantIDsByName(departments, parentName)
		if idSet == nil {
			idSet = parentSet
		} else {
			for id := range idSet {
				if _, ok := parentSet[id]; !ok {
					delete(idSet, id)
				}
			}
		}
	}
	if len(idSet) == 0 {
		return []WorkDepartment{}
	}

	filtered := make([]WorkDepartment, 0, len(departments))
	for _, department := range departments {
		if _, ok := idSet[department.ID]; ok {
			filtered = append(filtered, department)
		}
	}
	return filtered
}

func departmentDescendantIDsByName(departments []WorkDepartment, name string) map[int]struct{} {
	scopeRoots := map[int]struct{}{}
	for _, department := range departments {
		if !strings.Contains(department.Name, name) {
			continue
		}
		if rootID := departmentPageScopeRootID(department.Path); rootID > 0 {
			scopeRoots[rootID] = struct{}{}
		}
	}
	if len(scopeRoots) == 0 {
		return map[int]struct{}{}
	}

	ids := map[int]struct{}{}
	for _, department := range departments {
		for rootID := range scopeRoots {
			if strings.Contains(department.Path, "#"+strconv.Itoa(rootID)+"#") {
				ids[department.ID] = struct{}{}
				break
			}
		}
	}
	return ids
}

func departmentPageScopeRootID(path string) int {
	parts := strings.Split(path, "-")
	part := ""
	if len(parts) > 1 {
		part = parts[1]
	} else if len(parts) == 1 {
		part = parts[0]
	}
	id, err := strconv.Atoi(strings.Trim(part, "#"))
	if err != nil {
		return 0
	}
	return id
}

func workDepartmentPagePayload(departments []WorkDepartment, page int, perPage int) map[string]any {
	tree := workDepartmentPageTree(departments)
	total := len(tree)
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	offset := (page - 1) * perPage
	list := []map[string]any{}
	if offset < total {
		end := offset + perPage
		if end > total {
			end = total
		}
		list = tree[offset:end]
	}

	return map[string]any{
		"page": map[string]any{
			"perPage":   10,
			"total":     total,
			"totalPage": totalPage,
		},
		"list": list,
	}
}

func workDepartmentPageTree(departments []WorkDepartment) []map[string]any {
	items := make(map[int]map[string]any, len(departments))
	ordered := make([]int, 0, len(departments))
	for _, department := range departments {
		items[department.ID] = map[string]any{
			"id":           department.ID,
			"name":         department.Name,
			"level":        departmentLevelName(department.Level),
			"parentId":     department.ParentID,
			"departmentId": department.ID,
		}
		ordered = append(ordered, department.ID)
	}

	tree := make([]map[string]any, 0)
	for _, id := range ordered {
		item := items[id]
		parentID, _ := item["parentId"].(int)
		if parent, ok := items[parentID]; ok {
			children, _ := parent["children"].([]map[string]any)
			parent["children"] = append(children, item)
			continue
		}
		tree = append(tree, item)
	}

	for index := range tree {
		departmentPath := strconv.Itoa(index + 1)
		tree[index]["departmentPath"] = departmentPath
		if children, ok := tree[index]["children"].([]map[string]any); ok && len(children) > 0 {
			tree[index]["children"] = assignDepartmentPagePath(children, departmentPath)
		}
	}
	return tree
}

func assignDepartmentPagePath(items []map[string]any, path string) []map[string]any {
	for index := range items {
		departmentPath := path + "-" + strconv.Itoa(index+1)
		items[index]["departmentPath"] = departmentPath
		if children, ok := items[index]["children"].([]map[string]any); ok && len(children) > 0 {
			items[index]["children"] = assignDepartmentPagePath(children, departmentPath)
		}
	}
	return items
}

func departmentLevelName(level int) string {
	names := map[int]string{
		0:  "企业根部门",
		1:  "一级部门",
		2:  "二级部门",
		3:  "三级部门",
		4:  "四级部门",
		5:  "五级部门",
		6:  "六级部门",
		7:  "七级部门",
		8:  "八级部门",
		9:  "九级部门",
		10: "五级部门",
	}
	return names[level]
}

func queryIntStrictDefault(r *http.Request, name string, fallback int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func positiveQueryIntStrict(r *http.Request, name string, fallback int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if value <= 0 {
		return fallback, true
	}
	return value, true
}

func workEmployeeStatusName(status int) string {
	names := map[int]string{
		1: "已激活",
		2: "已禁用",
		4: "未激活",
		5: "退出企业",
	}
	return names[status]
}

func genderName(gender int) string {
	names := map[int]string{
		0: "未定义",
		1: "男",
		2: "女",
	}
	return names[gender]
}

func contactAuthName(contactAuth int) string {
	names := map[int]string{
		1: "是",
		2: "否",
	}
	return names[contactAuth]
}
