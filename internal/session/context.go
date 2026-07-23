package session

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	RequestSourceDashboard = 1
	RequestSourceWeChatApp = 2
)

type LoginCorpInfo struct {
	CorpIDs        []int
	WorkEmployeeID int
	RequestSource  int
}

type EmployeeByUserCorpFunc func(userID int, corpID int) int
type FirstEmployeeByUserFunc func(userID int) (corpID int, employeeID int, ok bool)

func ResolveLoginCorpInfo(
	headers http.Header,
	userID int,
	cacheValue string,
	employeeByUserCorp EmployeeByUserCorpFunc,
	firstEmployeeByUser FirstEmployeeByUserFunc,
) LoginCorpInfo {
	if headers.Get("mochat-source-type") == "wechat-app" {
		corpID := atoi(headers.Get("mochat-corp-id"))
		employeeID := 0
		if employeeByUserCorp != nil {
			employeeID = employeeByUserCorp(userID, corpID)
		}
		return LoginCorpInfo{
			CorpIDs:        []int{corpID},
			WorkEmployeeID: employeeID,
			RequestSource:  RequestSourceWeChatApp,
		}
	}

	if corpID, employeeID, ok := parseCachedCorpInfo(cacheValue); ok {
		return LoginCorpInfo{
			CorpIDs:        []int{corpID},
			WorkEmployeeID: employeeID,
			RequestSource:  RequestSourceDashboard,
		}
	}

	if firstEmployeeByUser != nil {
		if corpID, employeeID, ok := firstEmployeeByUser(userID); ok {
			return LoginCorpInfo{
				CorpIDs:        []int{corpID},
				WorkEmployeeID: employeeID,
				RequestSource:  RequestSourceDashboard,
			}
		}
	}

	return LoginCorpInfo{
		CorpIDs:        []int{},
		WorkEmployeeID: 0,
		RequestSource:  RequestSourceDashboard,
	}
}

func parseCachedCorpInfo(value string) (corpID int, employeeID int, ok bool) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	corpID = atoi(parts[0])
	employeeID = atoi(parts[1])
	return corpID, employeeID, true
}

func atoi(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
