package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) SidebarWorkbenchSummary(ctx context.Context, employee dashboard.SidebarEmployee) (dashboard.SidebarWorkbenchSummary, error) {
	var summary dashboard.SidebarWorkbenchSummary
	var avatar, departmentNames sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT
			employee.id,
			employee.name,
			employee.avatar,
			corp.name,
			COALESCE(GROUP_CONCAT(DISTINCT department.name ORDER BY department.`+"`order`"+` SEPARATOR 0x1f), ''),
			(SELECT COUNT(DISTINCT rel.contact_id)
			 FROM mc_work_contact_employee AS rel
			 JOIN mc_work_contact AS contact ON contact.id = rel.contact_id AND contact.corp_id = rel.corp_id AND contact.deleted_at IS NULL
			 WHERE rel.employee_id = employee.id AND rel.corp_id = employee.corp_id AND rel.deleted_at IS NULL),
			(SELECT COUNT(DISTINCT rel.contact_id)
			 FROM mc_work_contact_employee AS rel
			 JOIN mc_work_contact AS contact ON contact.id = rel.contact_id AND contact.corp_id = rel.corp_id AND contact.deleted_at IS NULL
			 WHERE rel.employee_id = employee.id AND rel.corp_id = employee.corp_id AND rel.deleted_at IS NULL AND DATE(rel.create_time) = CURRENT_DATE()),
			(SELECT COUNT(DISTINCT rel.contact_id)
			 FROM mc_work_contact_employee AS rel
			 JOIN mc_work_contact AS contact ON contact.id = rel.contact_id AND contact.corp_id = rel.corp_id AND contact.deleted_at IS NULL
			 JOIN mc_work_contact_tag_pivot AS pivot ON pivot.contact_id = rel.contact_id AND pivot.employee_id = employee.id AND pivot.deleted_at IS NULL
			 JOIN mc_work_contact_tag AS tag ON tag.id = pivot.contact_tag_id AND tag.corp_id = employee.corp_id AND tag.deleted_at IS NULL
			 WHERE rel.employee_id = employee.id AND rel.corp_id = employee.corp_id AND rel.deleted_at IS NULL),
			(SELECT COUNT(*) FROM mc_work_room AS room WHERE room.owner_id = employee.id AND room.corp_id = employee.corp_id AND room.deleted_at IS NULL),
			(SELECT COUNT(*) FROM mc_contact_sop_log AS contact_log WHERE contact_log.corp_id = employee.corp_id AND contact_log.employee = employee.wx_user_id),
			(SELECT COUNT(*) FROM mc_room_sop_log AS room_log WHERE room_log.corp_id = employee.corp_id AND room_log.employee = employee.wx_user_id AND room_log.state = 0),
			(SELECT COUNT(DISTINCT batch.record_id) FROM mc_contact_batch_add_import AS batch WHERE batch.employee_id = employee.id AND batch.corp_id = employee.corp_id AND batch.status IN (0, 1, 2) AND batch.deleted_at IS NULL)
		FROM mc_work_employee AS employee
		JOIN mc_corp AS corp ON corp.id = employee.corp_id AND corp.deleted_at IS NULL
		LEFT JOIN mc_work_employee_department AS employee_department ON employee_department.employee_id = employee.id AND employee_department.deleted_at IS NULL
		LEFT JOIN mc_work_department AS department ON department.id = employee_department.department_id AND department.corp_id = employee.corp_id AND department.deleted_at IS NULL
		WHERE employee.id = ? AND employee.corp_id = ? AND employee.deleted_at IS NULL
		GROUP BY employee.id, employee.name, employee.avatar, employee.wx_user_id, employee.corp_id, corp.name
	`, employee.ID, employee.CorpID).Scan(
		&summary.Employee.ID,
		&summary.Employee.Name,
		&avatar,
		&summary.Employee.CorpName,
		&departmentNames,
		&summary.Customers.Total,
		&summary.Customers.AddedToday,
		&summary.Customers.TaggedTotal,
		&summary.Customers.OwnedRoomTotal,
		&summary.Tasks.ContactSOPPending,
		&summary.Tasks.RoomSOPPending,
		&summary.Tasks.BatchAddPending,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SidebarWorkbenchSummary{}, nil
	}
	if err != nil {
		return dashboard.SidebarWorkbenchSummary{}, err
	}
	summary.Employee.Avatar = nullString(avatar)
	summary.Employee.DepartmentNames = splitSidebarList(nullString(departmentNames))
	return summary, nil
}

func (s *MySQLStore) SidebarContacts(ctx context.Context, employee dashboard.SidebarEmployee, filter dashboard.SidebarContactFilter) (dashboard.SidebarContactPage, error) {
	page := dashboard.SidebarContactPage{Page: filter.Page, PerPage: filter.PerPage, Items: []dashboard.SidebarContactListItem{}}
	where := []string{
		"rel.employee_id = ?",
		"rel.corp_id = ?",
		"rel.deleted_at IS NULL",
		"contact.deleted_at IS NULL",
	}
	args := []any{employee.ID, employee.CorpID}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, "(contact.name LIKE ? OR rel.remark LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	from := `
		FROM mc_work_contact_employee AS rel
		JOIN mc_work_contact AS contact ON contact.id = rel.contact_id AND contact.corp_id = rel.corp_id
		LEFT JOIN mc_work_contact_tag_pivot AS pivot ON pivot.contact_id = rel.contact_id AND pivot.employee_id = rel.employee_id AND pivot.deleted_at IS NULL
		LEFT JOIN mc_work_contact_tag AS tag ON tag.id = pivot.contact_tag_id AND tag.corp_id = rel.corp_id AND tag.deleted_at IS NULL
		WHERE ` + strings.Join(where, " AND ")
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT rel.contact_id) "+from, args...).Scan(&page.Total); err != nil {
		return dashboard.SidebarContactPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			contact.id,
			contact.wx_external_userid,
			contact.name,
			contact.avatar,
			rel.remark,
			rel.status,
			DATE_FORMAT(rel.create_time, '%Y-%m-%d %H:%i:%s'),
			COALESCE(GROUP_CONCAT(DISTINCT tag.name ORDER BY tag.name SEPARATOR 0x1f), '')
	`+from+`
		GROUP BY contact.id, contact.wx_external_userid, contact.name, contact.avatar, rel.remark, rel.status, rel.create_time
		ORDER BY rel.create_time DESC, contact.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.SidebarContactPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item dashboard.SidebarContactListItem
		var wxExternalUserID, name, avatar, remark, addedAt, tags sql.NullString
		if err := rows.Scan(&item.ID, &wxExternalUserID, &name, &avatar, &remark, &item.Status, &addedAt, &tags); err != nil {
			return dashboard.SidebarContactPage{}, err
		}
		item.WXExternalUserID = nullString(wxExternalUserID)
		item.Name = nullString(name)
		item.Avatar = nullString(avatar)
		item.Remark = nullString(remark)
		item.AddedAt = nullString(addedAt)
		item.Tags = splitSidebarList(nullString(tags))
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SidebarContactPage{}, err
	}
	return page, nil
}

func (s *MySQLStore) SidebarTasks(ctx context.Context, employee dashboard.SidebarEmployee, filter dashboard.SidebarTaskFilter) (dashboard.SidebarTaskPage, error) {
	page := dashboard.SidebarTaskPage{Page: filter.Page, PerPage: filter.PerPage, Items: []dashboard.SidebarTaskListItem{}}
	if filter.Kind == "contactSop" && filter.State == "done" {
		return page, nil
	}
	from, selectSQL, args, err := sidebarTaskQuery(employee, filter)
	if err != nil {
		return dashboard.SidebarTaskPage{}, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) "+from, args...).Scan(&page.Total); err != nil {
		return dashboard.SidebarTaskPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}
	page.TotalPage = (page.Total + filter.PerPage - 1) / filter.PerPage
	queryArgs := append(append([]any{}, args...), filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, selectSQL+from+" ORDER BY scheduled_at DESC, id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return dashboard.SidebarTaskPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item dashboard.SidebarTaskListItem
		if err := rows.Scan(&item.ID, &item.Kind, &item.Title, &item.SubjectName, &item.ScheduledAt, &item.State); err != nil {
			return dashboard.SidebarTaskPage{}, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SidebarTaskPage{}, err
	}
	return page, nil
}

func sidebarTaskQuery(employee dashboard.SidebarEmployee, filter dashboard.SidebarTaskFilter) (string, string, []any, error) {
	stateOperator := "= 0"
	stateText := "pending"
	if filter.State == "done" {
		stateOperator = "<> 0"
		stateText = "done"
	}
	switch filter.Kind {
	case "contactSop":
		return `
			FROM mc_contact_sop_log AS log
			JOIN mc_work_employee AS employee ON employee.id = ? AND employee.corp_id = ? AND employee.wx_user_id = log.employee AND employee.deleted_at IS NULL
			LEFT JOIN mc_contact_sop AS sop ON sop.id = log.contact_sop_id AND sop.corp_id = log.corp_id
			LEFT JOIN mc_work_contact AS contact ON contact.wx_external_userid = log.contact AND contact.corp_id = log.corp_id AND contact.deleted_at IS NULL
			WHERE log.corp_id = employee.corp_id
		`, `SELECT log.id, 'contactSop', COALESCE(sop.name, '个人客户 SOP'), COALESCE(contact.name, ''), COALESCE(DATE_FORMAT(log.created_at, '%Y-%m-%d %H:%i:%s'), ''), 'pending' `, []any{employee.ID, employee.CorpID}, nil
	case "roomSop":
		return `
			FROM mc_room_sop_log AS log
			JOIN mc_work_employee AS employee ON employee.id = ? AND employee.corp_id = ? AND employee.wx_user_id = log.employee AND employee.deleted_at IS NULL
			LEFT JOIN mc_room_sop AS sop ON sop.id = log.room_sop_id AND sop.corp_id = log.corp_id
			JOIN mc_work_room AS room ON room.id = log.room_id AND room.corp_id = log.corp_id AND room.deleted_at IS NULL
			WHERE log.corp_id = employee.corp_id AND log.state ` + stateOperator + `
		`, `SELECT log.id, 'roomSop', COALESCE(sop.name, '客户群 SOP'), room.name, COALESCE(DATE_FORMAT(log.created_at, '%Y-%m-%d %H:%i:%s'), ''), '` + stateText + `' `, []any{employee.ID, employee.CorpID}, nil
	case "batchAdd":
		pendingPredicate := "SUM(CASE WHEN batch.status IN (0, 1, 2) THEN 1 ELSE 0 END) > 0"
		if filter.State == "done" {
			pendingPredicate = "SUM(CASE WHEN batch.status IN (0, 1, 2) THEN 1 ELSE 0 END) = 0"
		}
		grouped := `
			FROM (
				SELECT batch.record_id AS id,
				       CONCAT('批量任务 #', batch.record_id) AS title,
				       CONCAT(COUNT(*), ' 位客户') AS subject_name,
				       COALESCE(DATE_FORMAT(MAX(batch.created_at), '%Y-%m-%d %H:%i:%s'), '') AS scheduled_at
				FROM mc_contact_batch_add_import AS batch
				WHERE batch.employee_id = ? AND batch.corp_id = ? AND batch.deleted_at IS NULL
				GROUP BY batch.record_id
				HAVING ` + pendingPredicate + `
			) AS grouped
		`
		return grouped, `SELECT grouped.id, 'batchAdd', grouped.title, grouped.subject_name, grouped.scheduled_at, '` + stateText + `' `, []any{employee.ID, employee.CorpID}, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported sidebar task kind %q", filter.Kind)
	}
}

func splitSidebarList(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, "\x1f")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}
