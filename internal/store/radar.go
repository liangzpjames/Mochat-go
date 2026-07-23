package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RadarPage(ctx context.Context, filter dashboard.RadarFilter) (dashboard.RadarPage, error) {
	filter.Page = positivePage(filter.Page)
	filter.PerPage = positivePerPage(filter.PerPage, 15)
	where, args := radarWhere(filter)

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_radar r "+where, args...).Scan(&total); err != nil {
		return dashboard.RadarPage{}, err
	}
	totalPage := 0
	if filter.PerPage > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage
	query := radarSelectPrefix() + where + " ORDER BY r.id DESC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), filter.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return dashboard.RadarPage{}, err
	}
	defer rows.Close()
	items, err := scanRadarRows(rows)
	if err != nil {
		return dashboard.RadarPage{}, err
	}
	return dashboard.RadarPage{Items: items, Total: total, TotalPage: totalPage, Page: filter.Page, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RadarByID(ctx context.Context, corpID int, id int) (dashboard.RadarItem, bool, error) {
	row := s.db.QueryRowContext(ctx, radarSelectPrefix()+`
		WHERE r.corp_id = ? AND r.id = ? AND r.deleted_at IS NULL
		LIMIT 1
	`, corpID, id)
	return scanRadar(row)
}

func (s *MySQLStore) CreateRadar(ctx context.Context, values dashboard.RadarWrite) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_radar
			(type, title, link, link_title, link_description, link_cover, pdf_name, pdf, article_type, article, employee_card, action_notice, dynamic_notice, contact_tags, tag_status, contact_grade, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, values.Type, values.Title, values.Link, values.LinkTitle, values.LinkDescription, values.LinkCover, values.PDFName, values.PDF, values.ArticleType,
		radarJSONOrArray(values.ArticleRaw), values.EmployeeCard, values.ActionNotice, values.DynamicNotice, radarJSONOrArray(values.ContactTagsRaw), values.TagStatus, radarJSONOrArray(values.ContactGradeRaw),
		tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) UpdateRadar(ctx context.Context, corpID int, id int, values dashboard.RadarWrite) (bool, error) {
	oldPaths, tenantID, found, err := s.radarStoragePaths(ctx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if values.HasType {
		sets = append(sets, "type = ?")
		args = append(args, values.Type)
	}
	if values.HasTitle {
		sets = append(sets, "title = ?")
		args = append(args, values.Title)
	}
	if values.HasLink {
		sets = append(sets, "link = ?")
		args = append(args, values.Link)
	}
	if values.HasLinkTitle {
		sets = append(sets, "link_title = ?")
		args = append(args, values.LinkTitle)
	}
	if values.HasLinkDescription {
		sets = append(sets, "link_description = ?")
		args = append(args, values.LinkDescription)
	}
	if values.HasLinkCover {
		sets = append(sets, "link_cover = ?")
		args = append(args, values.LinkCover)
	}
	if values.HasPDFName {
		sets = append(sets, "pdf_name = ?")
		args = append(args, values.PDFName)
	}
	if values.HasPDF {
		sets = append(sets, "pdf = ?")
		args = append(args, values.PDF)
	}
	if values.HasArticleType {
		sets = append(sets, "article_type = ?")
		args = append(args, values.ArticleType)
	}
	if values.HasArticle {
		sets = append(sets, "article = ?")
		args = append(args, radarJSONOrArray(values.ArticleRaw))
	}
	if values.HasEmployeeCard {
		sets = append(sets, "employee_card = ?")
		args = append(args, values.EmployeeCard)
	}
	if values.HasActionNotice {
		sets = append(sets, "action_notice = ?")
		args = append(args, values.ActionNotice)
	}
	if values.HasDynamicNotice {
		sets = append(sets, "dynamic_notice = ?")
		args = append(args, values.DynamicNotice)
	}
	if values.HasContactTags {
		sets = append(sets, "contact_tags = ?")
		args = append(args, radarJSONOrArray(values.ContactTagsRaw))
	}
	if values.HasTagStatus {
		sets = append(sets, "tag_status = ?")
		args = append(args, values.TagStatus)
	}
	if values.HasContactGrade {
		sets = append(sets, "contact_grade = ?")
		args = append(args, radarJSONOrArray(values.ContactGradeRaw))
	}
	args = append(args, corpID, id)
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_radar
		SET `+strings.Join(sets, ", ")+`
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	newPaths, _, _, err := s.radarStoragePaths(ctx, corpID, id)
	if err != nil {
		return true, err
	}
	reclaimPaths := storagePathsRemoved(oldPaths, newPaths)
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) DeleteRadar(ctx context.Context, corpID int, id int) (bool, error) {
	reclaimPaths, tenantID, found, err := s.radarStoragePaths(ctx, corpID, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_radar
		SET deleted_at = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, time.Now(), time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := s.reclaimSaaSStorageObjects(ctx, tenantID, reclaimPaths); err != nil {
		return true, err
	}
	return true, nil
}

func (s *MySQLStore) radarStoragePaths(ctx context.Context, corpID int, id int) ([]string, int, bool, error) {
	tenantID := 0
	linkCover := ""
	pdf := ""
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(link_cover, ''), COALESCE(pdf, ''), COALESCE(tenant_id, 0)
		FROM mc_radar
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, id).Scan(&linkCover, &pdf, &tenantID)
	if err == sql.ErrNoRows {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	return radarStoragePathsFromFields(linkCover, pdf), tenantID, true, nil
}

func radarStoragePathsFromFields(values ...string) []string {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if idx := strings.Index(value, "/static/"); idx >= 0 {
			value = value[idx+len("/static/"):]
		}
		value = strings.TrimPrefix(value, "static/")
		paths = append(paths, value)
	}
	return uniqueStorageRelativePaths(paths)
}

func (s *MySQLStore) CreateRadarChannel(ctx context.Context, corpID int, userID int, name string) (int, error) {
	tenantID, err := s.tenantIDByCorpID(ctx, corpID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_radar_channel (name, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, strings.TrimSpace(name), tenantID, corpID, userID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *MySQLStore) RadarChannelPage(ctx context.Context, corpID int, name string, page int, perPage int) (dashboard.RadarChannelPage, error) {
	page = positivePage(page)
	perPage = positivePerPage(perPage, 100)
	where := "WHERE c.corp_id = ? AND c.deleted_at IS NULL"
	args := []any{corpID}
	if strings.TrimSpace(name) != "" {
		where += " AND c.name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(name)+"%")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_radar_channel c "+where, args...).Scan(&total); err != nil {
		return dashboard.RadarChannelPage{}, err
	}
	totalPage := 0
	if perPage > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	offset := (page - 1) * perPage
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, COALESCE(c.name, ''), COALESCE(c.tenant_id, 0), COALESCE(c.corp_id, 0), COALESCE(c.create_user_id, 0), COALESCE(u.name, ''), c.created_at, c.updated_at
		FROM mc_radar_channel c
		LEFT JOIN mc_user u ON u.id = c.create_user_id AND u.deleted_at IS NULL
		`+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RadarChannelPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RadarChannelItem, 0)
	for rows.Next() {
		item, err := scanRadarChannelRow(rows)
		if err != nil {
			return dashboard.RadarChannelPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RadarChannelPage{}, err
	}
	return dashboard.RadarChannelPage{Items: items, Total: total, TotalPage: totalPage, Page: page, PerPage: perPage}, nil
}

func (s *MySQLStore) UpsertRadarChannelLink(ctx context.Context, values dashboard.RadarChannelLinkWrite) (dashboard.RadarChannelLinkItem, error) {
	if err := s.ensureRadarChannelLinkInputs(ctx, values); err != nil {
		return dashboard.RadarChannelLinkItem{}, err
	}
	tenantID, err := s.tenantIDByCorpID(ctx, values.CorpID)
	if err != nil {
		return dashboard.RadarChannelLinkItem{}, err
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO mc_radar_channel_link
			(radar_id, channel_id, link, employee_id, click_num, click_person_num, tenant_id, corp_id, create_user_id, created_at, updated_at)
		VALUES (?, ?, '', ?, 0, 0, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			deleted_at = NULL,
			create_user_id = VALUES(create_user_id),
			updated_at = VALUES(updated_at),
			id = LAST_INSERT_ID(id)
	`, values.RadarID, values.ChannelID, values.EmployeeID, tenantID, values.CorpID, values.CreateUserID, now, now)
	if err != nil {
		return dashboard.RadarChannelLinkItem{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return dashboard.RadarChannelLinkItem{}, err
	}
	item, found, err := s.radarChannelLinkByID(ctx, values.CorpID, int(id))
	if err != nil {
		return dashboard.RadarChannelLinkItem{}, err
	}
	if !found {
		return dashboard.RadarChannelLinkItem{}, sql.ErrNoRows
	}
	return item, nil
}

func (s *MySQLStore) UpdateRadarChannelLinkURL(ctx context.Context, corpID int, id int, link string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE mc_radar_channel_link
		SET link = ?, updated_at = ?
		WHERE corp_id = ? AND id = ? AND deleted_at IS NULL
	`, link, time.Now(), corpID, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *MySQLStore) RadarChannelLinkPage(ctx context.Context, corpID int, radarID int, page int, perPage int) (dashboard.RadarChannelLinkPage, error) {
	return s.radarChannelLinkPage(ctx, corpID, radarID, page, perPage)
}

func (s *MySQLStore) RadarOverview(ctx context.Context, corpID int, radarID int, codeType int) (dashboard.RadarOverview, error) {
	overview := dashboard.RadarOverview{RadarID: radarID}
	linkQuery := `
		SELECT COALESCE(SUM(click_num), 0), COALESCE(SUM(click_person_num), 0), COUNT(*)
		FROM mc_radar_channel_link
		WHERE corp_id = ? AND radar_id = ? AND deleted_at IS NULL
	`
	var linkClick, linkPerson int
	if err := s.db.QueryRowContext(ctx, linkQuery, corpID, radarID).Scan(&linkClick, &linkPerson, &overview.ChannelNum); err != nil {
		return dashboard.RadarOverview{}, err
	}
	recordWhere := "WHERE corp_id = ? AND radar_id = ? AND deleted_at IS NULL"
	recordArgs := []any{corpID, radarID}
	if codeType > 0 {
		recordWhere += " AND type = ?"
		recordArgs = append(recordArgs, codeType)
	}
	var recordClick, recordPerson int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COUNT(DISTINCT CASE
		          WHEN contact_id > 0 THEN CONCAT('c:', contact_id)
		          WHEN COALESCE(union_id, '') <> '' THEN CONCAT('u:', union_id)
		          ELSE CONCAT('n:', COALESCE(nickname, ''))
		       END)
		FROM mc_radar_record `+recordWhere, recordArgs...).Scan(&recordClick, &recordPerson); err != nil {
		return dashboard.RadarOverview{}, err
	}
	overview.ClickNum = maxInt(linkClick, recordClick)
	overview.ClickPersonNum = maxInt(linkPerson, recordPerson)
	return overview, nil
}

func (s *MySQLStore) RadarRecordPage(ctx context.Context, corpID int, radarID int, channelID int, page int, perPage int) (dashboard.RadarRecordPage, error) {
	page = positivePage(page)
	perPage = positivePerPage(perPage, 15)
	where := "WHERE r.corp_id = ? AND r.deleted_at IS NULL"
	args := []any{corpID}
	if radarID > 0 {
		where += " AND r.radar_id = ?"
		args = append(args, radarID)
	}
	if channelID > 0 {
		where += " AND r.channel_id = ?"
		args = append(args, channelID)
	}
	groupBy := "GROUP BY r.radar_id, radar.title, r.channel_id, ch.name, r.union_id, r.nickname, r.contact_id, r.employee_id, e.name, r.corp_id"
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT 1 FROM mc_radar_record r LEFT JOIN mc_radar radar ON radar.id = r.radar_id LEFT JOIN mc_radar_channel ch ON ch.id = r.channel_id LEFT JOIN mc_work_employee e ON e.id = r.employee_id "+where+" "+groupBy+") x", args...).Scan(&total); err != nil {
		return dashboard.RadarRecordPage{}, err
	}
	totalPage := 0
	if perPage > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	offset := (page - 1) * perPage
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT MAX(r.id), r.radar_id, COALESCE(radar.title, ''), r.channel_id, COALESCE(ch.name, ''), MAX(r.type),
		       COALESCE(r.union_id, ''), COALESCE(r.nickname, ''), COALESCE(MAX(r.avatar), ''), COALESCE(r.contact_id, 0), r.employee_id, COALESCE(e.name, ''),
		       COALESCE(GROUP_CONCAT(CONCAT(DATE_FORMAT(r.created_at, '%Y-%m-%d %H:%i:%s'), '|', COALESCE(r.content, '')) ORDER BY r.created_at DESC SEPARATOR '\n'), ''),
		       r.corp_id, COUNT(*), MAX(r.created_at), MAX(r.updated_at)
		FROM mc_radar_record r
		LEFT JOIN mc_radar radar ON radar.id = r.radar_id AND radar.deleted_at IS NULL
		LEFT JOIN mc_radar_channel ch ON ch.id = r.channel_id AND ch.deleted_at IS NULL
		LEFT JOIN mc_work_employee e ON e.id = r.employee_id AND e.deleted_at IS NULL
		`+where+`
		`+groupBy+`
		ORDER BY MAX(r.created_at) DESC, MAX(r.id) DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RadarRecordPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RadarRecordItem, 0)
	for rows.Next() {
		item, err := scanRadarRecordRow(rows)
		if err != nil {
			return dashboard.RadarRecordPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RadarRecordPage{}, err
	}
	return dashboard.RadarRecordPage{Items: items, Total: total, TotalPage: totalPage, Page: page, PerPage: perPage}, nil
}

func (s *MySQLStore) RadarChannelStats(ctx context.Context, corpID int, radarID int, page int, perPage int) (dashboard.RadarChannelLinkPage, error) {
	return s.radarChannelLinkPage(ctx, corpID, radarID, page, perPage)
}

func (s *MySQLStore) ensureRadarChannelLinkInputs(ctx context.Context, values dashboard.RadarChannelLinkWrite) error {
	total, err := s.countRows(ctx, `
		SELECT COUNT(*)
		FROM mc_radar
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.RadarID, values.CorpID)
	if err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("互动雷达不存在")
	}
	total, err = s.countRows(ctx, `
		SELECT COUNT(*)
		FROM mc_radar_channel
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.ChannelID, values.CorpID)
	if err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("雷达渠道不存在")
	}
	total, err = s.countRows(ctx, `
		SELECT COUNT(*)
		FROM mc_work_employee
		WHERE id = ? AND corp_id = ? AND deleted_at IS NULL
	`, values.EmployeeID, values.CorpID)
	if err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("员工不存在")
	}
	return nil
}

func (s *MySQLStore) radarChannelLinkPage(ctx context.Context, corpID int, radarID int, page int, perPage int) (dashboard.RadarChannelLinkPage, error) {
	page = positivePage(page)
	perPage = positivePerPage(perPage, 100)
	where := "WHERE l.corp_id = ? AND l.deleted_at IS NULL"
	args := []any{corpID}
	if radarID > 0 {
		where += " AND l.radar_id = ?"
		args = append(args, radarID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mc_radar_channel_link l "+where, args...).Scan(&total); err != nil {
		return dashboard.RadarChannelLinkPage{}, err
	}
	totalPage := 0
	if perPage > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	offset := (page - 1) * perPage
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := s.db.QueryContext(ctx, radarChannelLinkSelectPrefix()+where+`
		ORDER BY l.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.RadarChannelLinkPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RadarChannelLinkItem, 0)
	for rows.Next() {
		item, err := scanRadarChannelLinkRow(rows)
		if err != nil {
			return dashboard.RadarChannelLinkPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RadarChannelLinkPage{}, err
	}
	return dashboard.RadarChannelLinkPage{Items: items, Total: total, TotalPage: totalPage, Page: page, PerPage: perPage}, nil
}

func (s *MySQLStore) radarChannelLinkByID(ctx context.Context, corpID int, id int) (dashboard.RadarChannelLinkItem, bool, error) {
	row := s.db.QueryRowContext(ctx, radarChannelLinkSelectPrefix()+`
		WHERE l.corp_id = ? AND l.id = ? AND l.deleted_at IS NULL
		LIMIT 1
	`, corpID, id)
	item, err := scanRadarChannelLinkRow(row)
	if err == sql.ErrNoRows {
		return dashboard.RadarChannelLinkItem{}, false, nil
	}
	if err != nil {
		return dashboard.RadarChannelLinkItem{}, false, err
	}
	return item, true, nil
}

func radarWhere(filter dashboard.RadarFilter) (string, []any) {
	where := "WHERE r.corp_id = ? AND r.deleted_at IS NULL"
	args := []any{filter.CorpID}
	if filter.Type > 0 {
		where += " AND r.type = ?"
		args = append(args, filter.Type)
	}
	if strings.TrimSpace(filter.Title) != "" {
		like := "%" + strings.TrimSpace(filter.Title) + "%"
		where += " AND (r.title LIKE ? OR r.link_title LIKE ? OR r.link LIKE ?)"
		args = append(args, like, like, like)
	}
	return " " + where, args
}

func radarSelectPrefix() string {
	return `
		SELECT r.id, COALESCE(r.type, 0), COALESCE(r.title, ''), COALESCE(r.link, ''), COALESCE(r.link_title, ''), COALESCE(r.link_description, ''),
		       COALESCE(r.link_cover, ''), COALESCE(r.pdf_name, ''), COALESCE(r.pdf, ''), COALESCE(r.article_type, 0), COALESCE(r.article, JSON_ARRAY()),
		       COALESCE(r.employee_card, 0), COALESCE(r.action_notice, 0), COALESCE(r.dynamic_notice, 0), COALESCE(r.contact_tags, JSON_ARRAY()),
		       COALESCE(r.tag_status, 0), COALESCE(r.contact_grade, JSON_ARRAY()), COALESCE(r.tenant_id, 0), COALESCE(r.corp_id, 0), COALESCE(r.create_user_id, 0),
		       COALESCE(u.name, ''),
		       COALESCE((SELECT SUM(l.click_num) FROM mc_radar_channel_link l WHERE l.radar_id = r.id AND l.deleted_at IS NULL), 0),
		       COALESCE((SELECT SUM(l.click_person_num) FROM mc_radar_channel_link l WHERE l.radar_id = r.id AND l.deleted_at IS NULL), 0),
		       COALESCE((SELECT COUNT(*) FROM mc_radar_channel_link l WHERE l.radar_id = r.id AND l.deleted_at IS NULL), 0),
		       r.created_at, r.updated_at
		FROM mc_radar r
		LEFT JOIN mc_user u ON u.id = r.create_user_id AND u.deleted_at IS NULL
	`
}

func radarChannelLinkSelectPrefix() string {
	return `
		SELECT l.id, l.radar_id, COALESCE(r.title, ''), l.channel_id, COALESCE(c.name, ''), COALESCE(l.link, ''), l.employee_id, COALESCE(e.name, ''),
		       COALESCE(l.click_num, 0), COALESCE(l.click_person_num, 0), COALESCE(l.tenant_id, 0), COALESCE(l.corp_id, 0), COALESCE(l.create_user_id, 0), COALESCE(u.name, ''),
		       l.created_at, l.updated_at
		FROM mc_radar_channel_link l
		LEFT JOIN mc_radar r ON r.id = l.radar_id AND r.deleted_at IS NULL
		LEFT JOIN mc_radar_channel c ON c.id = l.channel_id AND c.deleted_at IS NULL
		LEFT JOIN mc_work_employee e ON e.id = l.employee_id AND e.deleted_at IS NULL
		LEFT JOIN mc_user u ON u.id = l.create_user_id AND u.deleted_at IS NULL
	`
}

type radarScanner interface {
	Scan(dest ...any) error
}

func scanRadar(scanner radarScanner) (dashboard.RadarItem, bool, error) {
	item, err := scanRadarRow(scanner)
	if err == sql.ErrNoRows {
		return dashboard.RadarItem{}, false, nil
	}
	if err != nil {
		return dashboard.RadarItem{}, false, err
	}
	return item, true, nil
}

func scanRadarRows(rows *sql.Rows) ([]dashboard.RadarItem, error) {
	items := make([]dashboard.RadarItem, 0)
	for rows.Next() {
		item, err := scanRadarRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRadarRow(scanner radarScanner) (dashboard.RadarItem, error) {
	var item dashboard.RadarItem
	var article, contactTags, contactGrade []byte
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&item.Link,
		&item.LinkTitle,
		&item.LinkDescription,
		&item.LinkCover,
		&item.PDFName,
		&item.PDF,
		&item.ArticleType,
		&article,
		&item.EmployeeCard,
		&item.ActionNotice,
		&item.DynamicNotice,
		&contactTags,
		&item.TagStatus,
		&contactGrade,
		&item.TenantID,
		&item.CorpID,
		&item.CreateUserID,
		&item.CreateUserName,
		&item.ClickNum,
		&item.ClickPersonNum,
		&item.ChannelNum,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return dashboard.RadarItem{}, err
	}
	item.ArticleRaw = string(article)
	item.ContactTagsRaw = string(contactTags)
	item.ContactGradeRaw = string(contactGrade)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRadarChannelRow(scanner radarScanner) (dashboard.RadarChannelItem, error) {
	var item dashboard.RadarChannelItem
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Name, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RadarChannelItem{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRadarChannelLinkRow(scanner radarScanner) (dashboard.RadarChannelLinkItem, error) {
	var item dashboard.RadarChannelLinkItem
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.RadarID, &item.RadarTitle, &item.ChannelID, &item.ChannelName, &item.Link, &item.EmployeeID, &item.EmployeeName, &item.ClickNum, &item.ClickPersonNum, &item.TenantID, &item.CorpID, &item.CreateUserID, &item.CreateUserName, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RadarChannelLinkItem{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanRadarRecordRow(scanner radarScanner) (dashboard.RadarRecordItem, error) {
	var item dashboard.RadarRecordItem
	var rawClickInfo string
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.RadarID, &item.RadarTitle, &item.ChannelID, &item.ChannelName, &item.Type, &item.UnionID, &item.Nickname, &item.Avatar, &item.ContactID, &item.EmployeeID, &item.EmployeeName, &rawClickInfo, &item.CorpID, &item.ClickNum, &createdAt, &updatedAt)
	if err != nil {
		return dashboard.RadarRecordItem{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	item.ClickInfo = parseRadarClickInfo(rawClickInfo)
	if len(item.ClickInfo) > 0 {
		item.Content = item.ClickInfo[0].Content
	}
	return item, nil
}

func parseRadarClickInfo(raw string) []dashboard.RadarClickInfo {
	lines := strings.Split(raw, "\n")
	items := make([]dashboard.RadarClickInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		info := dashboard.RadarClickInfo{CreatedAt: parts[0]}
		if len(parts) == 2 {
			info.Content = parts[1]
		}
		items = append(items, info)
	}
	return items
}

func radarJSONOrArray(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]"
	}
	return raw
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
