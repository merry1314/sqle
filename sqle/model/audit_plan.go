package model

import (
	"encoding/json"
	e "errors"
	"fmt"
	"strings"
	"time"

	"github.com/actiontech/sqle/sqle/errors"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/actiontech/sqle/sqle/utils"

	"gorm.io/gorm"
)

type AuditPlan struct {
	Model
	ProjectId        ProjectUID    `gorm:"index; not null"`
	Name             string        `json:"name" gorm:"not null;index"`
	CronExpression   string        `json:"cron_expression" gorm:"not null;type:varchar(255)"`
	DBType           string        `json:"db_type" gorm:"not null;type:varchar(255)"`
	Token            string        `json:"token" gorm:"not null;type:varchar(255)"`
	InstanceName     string        `json:"instance_name" gorm:"type:varchar(255)"`
	CreateUserID     string        `json:"create_user_id" gorm:"type:varchar(255)"`
	InstanceDatabase string        `json:"instance_database" gorm:"type:varchar(255)"`
	Type             string        `json:"type" gorm:"type:varchar(255)"`
	RuleTemplateName string        `json:"rule_template_name" gorm:"type:varchar(255)"`
	Params           params.Params `json:"params" gorm:"type:json"`

	NotifyInterval      int    `json:"notify_interval" gorm:"default:10"`
	NotifyLevel         string `json:"notify_level" gorm:"default:'warn';type:varchar(255)"`
	EnableEmailNotify   bool   `json:"enable_email_notify"`
	EnableWebHookNotify bool   `json:"enable_web_hook_notify"`
	WebHookURL          string `json:"web_hook_url" gorm:"type:varchar(255)"`
	WebHookTemplate     string `json:"web_hook_template" gorm:"type:varchar(255)"`

	ProjectStatus string `gorm:"default:'active';type:varchar(255)"` // dms-todo: 暂时将项目状态放在这里

	// CreateUser    *User             // TODO 移除 `gorm:"foreignkey:CreateUserId"`
	Instance      *Instance         `gorm:"foreignkey:Name;references:InstanceName"`
	AuditPlanSQLs []*AuditPlanSQLV2 `gorm:"foreignkey:AuditPlanID"`
}

type AuditPlanSQLV2 struct {
	Model

	// add unique index on fingerprint and audit_plan_id
	// it's done by AutoMigrate() because gorm can't create index on TEXT column directly by tag.
	AuditPlanID    uint   `json:"audit_plan_id" gorm:"not null;uniqueIndex:uniq_audit_plan_sqls_v2_audit_plan_id_fingerprint_md5"`
	Fingerprint    string `json:"fingerprint" gorm:"type:mediumtext;not null"`
	FingerprintMD5 string `json:"fingerprint_md5" gorm:"type:varchar(512);column:fingerprint_md5;not null;uniqueIndex:uniq_audit_plan_sqls_v2_audit_plan_id_fingerprint_md5"`
	SQLContent     string `json:"sql" gorm:"type:mediumtext;not null"`
	Info           JSON   `gorm:"type:json"`
	Schema         string `json:"schema" gorm:"type:varchar(512);not null"`
}

type BlacklistFilterType string

const (
	FilterTypeSQL      BlacklistFilterType = "sql"
	FilterTypeFpSQL    BlacklistFilterType = "fp_sql"
	FilterTypeIP       BlacklistFilterType = "ip"
	FilterTypeCIDR     BlacklistFilterType = "cidr"
	FilterTypeHost     BlacklistFilterType = "host"
	FilterTypeInstance BlacklistFilterType = "instance"
	FilterTypeDbUser   BlacklistFilterType = "db_user"
)

type BlackListAuditPlanSQL struct {
	Model
	ProjectId     ProjectUID          `gorm:"index; not null"`
	FilterContent string              `json:"filter_content" gorm:"type:varchar(3000);not null;"`
	Desc          string              `json:"desc" gorm:"type:varchar(512)"`
	FilterType    BlacklistFilterType `json:"filter_type" gorm:"type:enum('sql','fp_sql','ip','cidr','host','instance','db_user');default:'sql';not null;"`
	MatchedCount  uint                `json:"matched_count" gorm:"default:0"`
	LastMatchTime *time.Time          `json:"last_match_time"`
}

func (a BlackListAuditPlanSQL) TableName() string {
	return "black_list_audit_plan_sqls"
}

func (s *Storage) GetBlacklistByID(projectID ProjectUID, id string) (*BlackListAuditPlanSQL, bool, error) {
	bl := &BlackListAuditPlanSQL{}
	err := s.db.Model(BlackListAuditPlanSQL{}).Where("project_id = ? AND id = ?", projectID, id).First(bl).Error
	if e.Is(err, gorm.ErrRecordNotFound) {
		return bl, false, nil
	}
	return bl, true, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetBlackListByProjectID(projectID ProjectUID) ([]*BlackListAuditPlanSQL, error) {
	var blackListAPS []*BlackListAuditPlanSQL
	err := s.db.Model(BlackListAuditPlanSQL{}).Where("project_id = ?", projectID).Find(&blackListAPS).Error
	return blackListAPS, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetBlacklistList(projectID ProjectUID, FilterType BlacklistFilterType, fuzzySearchContent string, pageIndex, pageSize uint32) ([]*BlackListAuditPlanSQL, uint64, error) {
	var count int64
	var blackListAPS []*BlackListAuditPlanSQL
	query := s.db.Model(BlackListAuditPlanSQL{}).Where("project_id = ?", projectID)
	if FilterType != "" {
		query = query.Where("filter_type = ?", FilterType)
	}
	if fuzzySearchContent != "" {
		query = query.Where("filter_content LIKE ?", "%"+fuzzySearchContent+"%")
	}
	err := query.Count(&count).Error
	if err != nil {
		return blackListAPS, uint64(count), errors.New(errors.ConnectStorageError, err)
	}

	if count == 0 {
		return blackListAPS, uint64(count), errors.New(errors.ConnectStorageError, err)
	}

	err = query.Offset(int((pageIndex - 1) * pageSize)).Limit(int(pageSize)).Order("id desc").Find(&blackListAPS).Error
	return blackListAPS, uint64(count), errors.New(errors.ConnectStorageError, err)
}

// GetBlacklistByProjectIDAndFilterType
func (s *Storage) GetBlacklistByProjectIDAndFilterType(projectID ProjectUID, filterType BlacklistFilterType) ([]*BlackListAuditPlanSQL, error) {
	var blackListAPS []*BlackListAuditPlanSQL

	err := s.db.Model(BlackListAuditPlanSQL{}).
		Where("project_id = ? AND filter_type = ?", projectID, filterType).
		Find(&blackListAPS).Error
	if err != nil {
		return nil, errors.New(errors.ConnectStorageError, err)
	}
	
	return blackListAPS, nil
}

func (s *Storage) BatchUpdateBlackListCount(matchedIdCount map[uint]uint, lastMatchedTime time.Time) error {
	countIdList := make(map[uint] /*count*/ []uint /*blacklist id list*/)
	for id, count := range matchedIdCount {
		countIdList[count] = append(countIdList[count], id)
	}

	for count, idList := range countIdList {
		m := map[string]interface{}{
			"matched_count":   gorm.Expr("matched_count + ?", count),
			"last_match_time": lastMatchedTime,
		}

		err := s.db.Model(BlackListAuditPlanSQL{}).Where("id in (?)", idList).Updates(m).Error
		if err != nil {
			return errors.New(errors.ConnectStorageError, err)
		}
	}

	return nil
}

func (a AuditPlanSQLV2) TableName() string {
	return "audit_plan_sqls_v2"
}

func (a *AuditPlanSQLV2) GetFingerprintMD5() string {
	if a.FingerprintMD5 != "" {
		return a.FingerprintMD5
	}
	// 为了区分具有相同Fingerprint但Schema不同的SQL，在这里加入Schema信息进行区分
	sqlIdentityJSON, _ := json.Marshal(
		struct {
			Fingerprint string
			Schema      string
		}{
			Fingerprint: a.Fingerprint,
			Schema:      a.Schema,
		},
	)
	return utils.Md5String(string(sqlIdentityJSON))
}

// BeforeSave is a hook implement gorm model before exec create.
func (a *AuditPlanSQLV2) BeforeSave(tx *gorm.DB) error {
	tx.Statement.SetColumn("FingerprintMD5", a.GetFingerprintMD5())
	return nil
}

func (s *Storage) GetAuditPlans() ([]*AuditPlan, error) {
	var aps []*AuditPlan
	err := s.db.Model(AuditPlan{}).Find(&aps).Error
	return aps, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetActiveAuditPlans() ([]*AuditPlan, error) {
	var aps []*AuditPlan
	err := s.db.Model(AuditPlan{}).
		Where("project_status = ?", ProjectStatusActive).
		Find(&aps).Error
	return aps, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetAuditPlanByName(name string) (*AuditPlan, bool, error) {
	ap := &AuditPlan{}
	err := s.db.Model(AuditPlan{}).Where("name = ?", name).First(ap).Error
	if err == gorm.ErrRecordNotFound {
		return ap, false, nil
	}
	return ap, true, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetAuditPlanById(id uint) (*AuditPlan, bool, error) {
	ap := &AuditPlan{}
	err := s.db.Model(AuditPlan{}).Where("id = ?", id).First(ap).Error
	if err == gorm.ErrRecordNotFound {
		return ap, false, nil
	}
	return ap, true, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetActiveAuditPlanById(id uint) (*AuditPlan, bool, error) {
	ap := &AuditPlan{}
	err := s.db.Model(AuditPlan{}).
		Where("project_status = ?", ProjectStatusActive).
		Where("id = ?", id).First(ap).Error
	if err == gorm.ErrRecordNotFound {
		return ap, false, nil
	}
	return ap, true, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetAuditPlanFromProjectByName(projectId, AuditPlanName string) (*AuditPlan, bool, error) {
	ap := &AuditPlan{}
	err := s.db.Model(AuditPlan{}).Where("project_id = ? AND name = ?", projectId, AuditPlanName).First(ap).Error
	if err == gorm.ErrRecordNotFound {
		return ap, false, nil
	}
	return ap, true, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetAuditPlanReportByID(auditPlanId, id uint) (*AuditPlanReportV2, bool, error) {
	ap := &AuditPlanReportV2{}
	err := s.db.Model(AuditPlanReportV2{}).Where("id = ? AND audit_plan_id = ?", id, auditPlanId).Preload("AuditPlan").First(ap).Error
	if err == gorm.ErrRecordNotFound {
		return ap, false, nil
	}
	return ap, true, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetAuditPlanSQLs(auditPlanId uint) ([]*AuditPlanSQLV2, error) {
	var sqls []*AuditPlanSQLV2
	err := s.db.Model(AuditPlanSQLV2{}).Where("audit_plan_id = ?", auditPlanId).Find(&sqls).Error
	return sqls, errors.New(errors.ConnectStorageError, err)
}
func (s *Storage) GetAuditPlanSQLsV2Unaudit(auditPlanId uint) ([]*SQLManageRecord, error) {
	var sqls []*SQLManageRecord
	err := s.db.Model(&SQLManageRecord{}).Where("source_id = ? AND audit_level IS NULL", auditPlanId).Find(&sqls).Error
	return sqls, errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) GetLatestStartTimeAuditPlanSQL(auditPlanId uint) (string, error) {
	var info = struct {
		StartTime string `gorm:"column:max_start_time"`
	}{}
	err := s.db.Raw(`SELECT MAX(STR_TO_DATE(JSON_UNQUOTE(JSON_EXTRACT(info, '$.start_time')), '%Y-%m-%dT%H:%i:%s.%fZ')) 
					AS max_start_time FROM audit_plan_sqls_v2 WHERE audit_plan_id = ?`, auditPlanId).Scan(&info).Error
	return info.StartTime, err
}

func (s *Storage) OverrideAuditPlanSQLsV2(auditPlanId uint, sqls []*SQLManageRecord) error {
	err := s.db.Unscoped().
		Model(AuditPlanSQLV2{}).
		Where("audit_plan_id = ?", auditPlanId).
		Delete(&AuditPlanSQLV2{}).Error
	if err != nil {
		return errors.New(errors.ConnectStorageError, err)
	}
	raw, args := getBatchInsertRawSQLV2(auditPlanId, sqls)
	return errors.New(errors.ConnectStorageError, s.db.Exec(fmt.Sprintf("%v;", raw), args...).Error)
}

func (s *Storage) OverrideAuditPlanSQLs(auditPlanId uint, sqls []*AuditPlanSQLV2) error {
	err := s.db.Unscoped().
		Model(AuditPlanSQLV2{}).
		Where("audit_plan_id = ?", auditPlanId).
		Delete(&AuditPlanSQLV2{}).Error
	if err != nil {
		return errors.New(errors.ConnectStorageError, err)
	}
	raw, args := getBatchInsertRawSQL(auditPlanId, sqls)
	return errors.New(errors.ConnectStorageError, s.db.Exec(fmt.Sprintf("%v;", raw), args...).Error)
}

func (s *Storage) UpdateDefaultAuditPlanSQLs(auditPlanId uint, sqls []*AuditPlanSQLV2) error {
	raw, args := getBatchInsertRawSQL(auditPlanId, sqls)
	// counter column is a accumulate value when update.
	raw += `ON DUPLICATE KEY UPDATE 
	sql_content = VALUES(sql_content), 
	info        = JSON_SET(
		COALESCE(info, '{}'),
		'$.counter', CAST(
			COALESCE(JSON_EXTRACT(values(info), '$.counter'), 0) 
			+ COALESCE(JSON_EXTRACT(info, '$.counter'), 0) 
			AS SIGNED
		),
		'$.last_receive_timestamp', JSON_EXTRACT(values(info), '$.last_receive_timestamp')
		);`

	return errors.New(errors.ConnectStorageError, s.db.Exec(raw, args...).Error)
}

func (s *Storage) UpdateSlowLogAuditPlanSQLs(auditPlanId uint, sqls []*AuditPlanSQLV2) error {
	raw, args := getBatchInsertRawSQL(auditPlanId, sqls)
	/*
		counter 每次累加传入的count，新的count=记录中的count+传入的count
		last_receive_timestamp 记录最后收到的时间戳，直接更新
		query_time_avg 计算增加后的平均时间，的计算公式为：
		(记录中的count*记录的平均时间+传入的count*传入的平均时间)/(记录中的count+传入的count)
		格式为12位浮点型小数点后保存6位，其中如果记录中没有平均时间，则直接设置为传入的平均时间，
		因为同一个sql的执行时间会随着sql的执行次数的增加而趋于收敛，所以这里我们直接设置为传入的平均时间。
		如果传入没有平均时间，则是因为老版本Scannerd没有传入该值，因此把平均时间则设定为0，
		为了保证计算中分母不为0，所以当找不到counter的时候，设置传入的counter为1
		query_time_max 比较并更新传入和记录中该字段的较大值
		first_query_at 记录该指纹的第一个日志记录的时间，为了兼容没有该功能的Scannerd，如果记录中无该值或该值为null，则更新为传入值，传入值为空时，更新为null，传入值不为空时更新为对应值
		db_user 目前暂时保存为第一次执行该SQL的数据库用户，更新逻辑与first_query_at一致
	*/
	raw += `ON DUPLICATE KEY UPDATE 
		sql_content = VALUES(sql_content), 
		info = JSON_SET(
			COALESCE(info, '{}'),
			'$.counter', CAST(
				COALESCE(JSON_EXTRACT(values(info), '$.counter'), 0) 
				+ COALESCE(JSON_EXTRACT(info, '$.counter'), 0) 
				AS SIGNED
			),
			'$.last_receive_timestamp', JSON_EXTRACT(values(info), '$.last_receive_timestamp'),
			'$.query_time_max', GREATEST(
				CAST(COALESCE(JSON_EXTRACT(info, '$.query_time_max'), 0) AS DECIMAL(12,6)),
				CAST(COALESCE(JSON_EXTRACT(values(info), '$.query_time_max'), 0) AS DECIMAL(12,6))
			),
			'$.query_time_avg', CAST(
			(
				COALESCE(JSON_EXTRACT(info, '$.query_time_avg'), JSON_EXTRACT(values(info), '$.query_time_avg'))*COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
				+COALESCE(JSON_EXTRACT(values(info), '$.query_time_avg'), 0)*COALESCE(JSON_EXTRACT(values(info), '$.counter'), 0)
			)/(
				COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
				+COALESCE(JSON_EXTRACT(values(info), '$.counter'), 1)
			)
				AS DECIMAL(12,6)
			),
			'$.row_examined_avg', CAST(
			(
				COALESCE(JSON_EXTRACT(info, '$.row_examined_avg'), JSON_EXTRACT(values(info), '$.row_examined_avg'))*COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
				+COALESCE(JSON_EXTRACT(values(info), '$.row_examined_avg'), 0)*COALESCE(JSON_EXTRACT(values(info), '$.counter'), 0)
			)/(
				COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
				+COALESCE(JSON_EXTRACT(values(info), '$.counter'), 1)
			)
				AS DECIMAL(65,6)
			),
			'$.first_query_at', IF(
				JSON_TYPE(JSON_EXTRACT(info, '$.first_query_at'))="NULL",
				JSON_EXTRACT(values(info), '$.first_query_at'),
				JSON_EXTRACT(info, '$.first_query_at')
			),
			'$.db_user', IF(
				JSON_TYPE(JSON_EXTRACT(info, '$.db_user'))="NULL",
				JSON_EXTRACT(values(info), '$.db_user'),
				JSON_EXTRACT(info, '$.db_user')
			),
            '$.endpoints', JSON_MERGE(
                JSON_EXTRACT(info, '$.endpoints'),
			    JSON_EXTRACT(VALUES(info), '$.endpoints')
            )
	  	);`

	return errors.New(errors.ConnectStorageError, s.db.Exec(raw, args...).Error)
}

func (s *Storage) UpdateSlowLogCollectAuditPlanSQLsV2(auditPlanId uint, sqls []*SQLManageRecord) error {
	raw, args := getBatchInsertRawSQLV2(auditPlanId, sqls)
	// counter column is a accumulate value when update.
	raw += `
ON DUPLICATE KEY UPDATE sql_text = VALUES(sql_text),
                        info        = JSON_SET(COALESCE(info, '{}'),
											  '$.counter', CAST(COALESCE(JSON_EXTRACT(values(info), '$.counter'), 0) +
                                                                COALESCE(JSON_EXTRACT(info, '$.counter'), 0) AS SIGNED),
                                              '$.last_receive_timestamp',
                                              JSON_EXTRACT(values(info), '$.last_receive_timestamp'),
											  '$.average_query_time',
											  CAST(
												((JSON_EXTRACT(info, '$.average_query_time') + 0) * (JSON_EXTRACT(info, '$.counter'))
												+ (JSON_EXTRACT(VALUES(info), '$.average_query_time') + 0) * (JSON_EXTRACT(VALUES(info), '$.counter')))
												/ (JSON_EXTRACT(info, '$.counter') + JSON_EXTRACT(VALUES(info), '$.counter'))
												AS UNSIGNED
											  ),
											  '$.row_examined_avg', CAST(
                                                       (
                                                           COALESCE(JSON_EXTRACT(info, '$.row_examined_avg'),
                                                                    JSON_EXTRACT(VALUES(info), '$.row_examined_avg')) *
                                                           COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
                                                               +
                                                           COALESCE(JSON_EXTRACT(VALUES(info), '$.row_examined_avg'), 0) *
                                                           COALESCE(JSON_EXTRACT(VALUES(info), '$.counter'), 0)
                                                           ) / (
                                                           COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
                                                               + COALESCE(JSON_EXTRACT(VALUES(info), '$.counter'), 1)
                                                           )
                                                   AS DECIMAL(65,6)),
											  '$.start_time',
											  JSON_EXTRACT(values(info), '$.start_time'));`

	return errors.New(errors.ConnectStorageError, s.db.Exec(raw, args...).Error)

}

func (s *Storage) UpdateSlowLogCollectAuditPlanSQLs(auditPlanId uint, sqls []*AuditPlanSQLV2) error {
	raw, args := getBatchInsertRawSQL(auditPlanId, sqls)
	// counter column is a accumulate value when update.
	raw += `
ON DUPLICATE KEY UPDATE sql_content = VALUES(sql_content),
                        info        = JSON_SET(COALESCE(info, '{}'),
											  '$.counter', CAST(COALESCE(JSON_EXTRACT(values(info), '$.counter'), 0) +
                                                                COALESCE(JSON_EXTRACT(info, '$.counter'), 0) AS SIGNED),
                                              '$.last_receive_timestamp',
                                              JSON_EXTRACT(values(info), '$.last_receive_timestamp'),
											  '$.average_query_time',
											  CAST(
												((JSON_EXTRACT(info, '$.average_query_time') + 0) * (JSON_EXTRACT(info, '$.counter'))
												+ (JSON_EXTRACT(VALUES(info), '$.average_query_time') + 0) * (JSON_EXTRACT(VALUES(info), '$.counter')))
												/ (JSON_EXTRACT(info, '$.counter') + JSON_EXTRACT(VALUES(info), '$.counter'))
												AS UNSIGNED
											  ),
											  '$.row_examined_avg', CAST(
                                                       (
                                                           COALESCE(JSON_EXTRACT(info, '$.row_examined_avg'),
                                                                    JSON_EXTRACT(VALUES(info), '$.row_examined_avg')) *
                                                           COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
                                                               +
                                                           COALESCE(JSON_EXTRACT(VALUES(info), '$.row_examined_avg'), 0) *
                                                           COALESCE(JSON_EXTRACT(VALUES(info), '$.counter'), 0)
                                                           ) / (
                                                           COALESCE(JSON_EXTRACT(info, '$.counter'), 0)
                                                               + COALESCE(JSON_EXTRACT(VALUES(info), '$.counter'), 1)
                                                           )
                                                   AS DECIMAL(65,6)),
											  '$.start_time',
											  JSON_EXTRACT(values(info), '$.start_time'));`

	return errors.New(errors.ConnectStorageError, s.db.Exec(raw, args...).Error)

}

func getBatchInsertRawSQLV2(auditPlanId uint, sqls []*SQLManageRecord) (raw string, args []interface{}) {
	pattern := make([]string, 0, len(sqls))
	for _, sql := range sqls {
		pattern = append(pattern, "(?, ?, ?, ?, ?, ?,?, ?, ?)")
		args = append(args, "audit_plan", auditPlanId, sql.ProjectId, sql.InstanceID, sql.SchemaName, sql.SqlFingerprint, sql.SqlText, sql.Info, sql.GetFingerprintMD5())
	}
	raw = fmt.Sprintf("INSERT INTO `sql_manage_records` (`source`,`source_id`,`project_id`,`instance_name`,`schema_name`,`sql_fingerprint`, `sql_text`, `info`,`proj_fp_source_inst_schema_md5`) VALUES %s",
		strings.Join(pattern, ", "))
	return
}

func getBatchInsertRawSQL(auditPlanId uint, sqls []*AuditPlanSQLV2) (raw string, args []interface{}) {
	pattern := make([]string, 0, len(sqls))
	for _, sql := range sqls {
		pattern = append(pattern, "(?, ?, ?, ?, ?, ?)")
		args = append(args, auditPlanId, sql.GetFingerprintMD5(), sql.Fingerprint, sql.SQLContent, sql.Info, sql.Schema)
	}
	raw = fmt.Sprintf("INSERT INTO `audit_plan_sqls_v2` (`audit_plan_id`,`fingerprint_md5`, `fingerprint`, `sql_content`, `info`, `schema`) VALUES %s",
		strings.Join(pattern, ", "))
	return
}

func (s *Storage) UpdateAuditPlanByName(name string, attrs map[string]interface{}) error {
	err := s.db.Model(AuditPlan{}).Where("name = ?", name).Updates(attrs).Error
	return errors.New(errors.ConnectStorageError, err)
}

func (s *Storage) UpdateAuditPlanById(id uint, attrs map[string]interface{}) error {
	err := s.db.Model(AuditPlan{}).Where("id = ?", id).Updates(attrs).Error
	return errors.New(errors.ConnectStorageError, err)
}

// func (s *Storage) GetAuditPlanTotalByProjectName(projectName string) (uint64, error) {
// 	var count uint64
// 	err := s.db.
// 		Table("audit_plans").
// 		Joins("LEFT JOIN projects ON audit_plans.project_id = projects.id").
// 		Where("projects.name = ?", projectName).
// 		Where("audit_plans.deleted_at IS NULL").
// 		Count(&count).
// 		Error
// 	return count, errors.ConnectStorageErrWrapper(err)
// }

// func (s *Storage) GetAuditPlanIDsByProjectName(projectName string) ([]uint, error) {
// 	ids := []struct {
// 		ID uint `json:"id"`
// 	}{}
// 	err := s.db.Table("audit_plans").
// 		Select("audit_plans.id").
// 		Joins("LEFT JOIN projects ON projects.id = audit_plans.project_id").
// 		Where("projects.name = ?", projectName).
// 		Find(&ids).Error

// 	resp := []uint{}
// 	for _, id := range ids {
// 		resp = append(resp, id.ID)
// 	}

// 	return resp, errors.ConnectStorageErrWrapper(err)
// }

// GetLatestAuditPlanIds 获取所有变更过的记录，包括删除
func (s *Storage) GetLatestAuditPlanRecords(after time.Time) ([]*AuditPlan, error) {
	var aps []*AuditPlan
	err := s.db.Unscoped().Model(AuditPlan{}).Select("id, updated_at").Where("updated_at > ?", after).Order("updated_at").Find(&aps).Error
	return aps, errors.New(errors.ConnectStorageError, err)
}

type RiskAuditPlan struct {
	ReportId       uint       `json:"report_id"`
	AuditPlanName  string     `json:"audit_plan_name"`
	ReportCreateAt *time.Time `json:"report_create_at"`
	RiskSqlCOUNT   uint       `json:"risk_sql_count"`
}

func (s *Storage) GetRiskAuditPlan(projectUid string) ([]*RiskAuditPlan, error) {
	var RiskAuditPlans []*RiskAuditPlan
	err := s.db.Model(AuditPlan{}).
		Select(`reports.id report_id, audit_plans.name audit_plan_name, reports.created_at report_create_at, 
				count(case when JSON_TYPE(report_sqls.audit_results)<>'NULL' then 1 else null end) risk_sql_count`).
		Joins("left join audit_plan_reports_v2 reports on audit_plans.id=reports.audit_plan_id").
		Joins("left join audit_plan_report_sqls_v2 report_sqls on report_sqls.audit_plan_report_id=reports.id").
		Where("reports.score<60 and audit_plans.project_id=? and audit_plans.deleted_at is NULL", projectUid).
		Group("audit_plans.name, reports.created_at, audit_plans.created_at, reports.id").
		Order("reports.created_at desc").Scan(&RiskAuditPlans).Error

	if err != nil {
		return nil, errors.ConnectStorageErrWrapper(err)
	}
	return RiskAuditPlans, nil

}

// 使用子查询获取最新的report时间，然后再获取最新report的sql数量和触发规则的sql数量
func (s *Storage) GetAuditPlanSQLCountAndTriggerRuleCountByProject(projectUid string) (SqlCountAndTriggerRuleCount, error) {
	sqlCountAndTriggerRuleCount := SqlCountAndTriggerRuleCount{}
	subQuery := s.db.Model(&AuditPlan{}).
		Select("audit_plans.id as audit_plan_id, MAX(audit_plan_reports_v2.created_at) as latest_created_at").
		Joins("left join audit_plan_reports_v2 on audit_plan_reports_v2.audit_plan_id=audit_plans.id").
		Where("audit_plans.project_id=? and audit_plans.deleted_at is null and audit_plan_reports_v2.id is not null", projectUid).
		Group("audit_plans.id")

	err := s.db.Model(&AuditPlan{}).
		Select("count(report_sqls.id) sql_count, count(case when JSON_TYPE(report_sqls.audit_results)<>'NULL' then 1 else null end) trigger_rule_count").
		Joins("left join audit_plan_reports_v2 reports on reports.audit_plan_id=audit_plans.id").
		Joins("left join audit_plan_report_sqls_v2 report_sqls on report_sqls.audit_plan_report_id=reports.id").
		Joins("join (?) as sq on audit_plans.id=sq.audit_plan_id and reports.created_at=sq.latest_created_at", subQuery).
		Where("audit_plans.project_id=? and audit_plans.deleted_at is null", projectUid).
		Scan(&sqlCountAndTriggerRuleCount).Error

	return sqlCountAndTriggerRuleCount, errors.ConnectStorageErrWrapper(err)
}

type DBTypeAuditPlanCount struct {
	DbType         string `json:"db_type"`
	Type           string `json:"type"`
	AuditPlanCount uint   `json:"audit_plan_count"`
}

func (s *Storage) GetDBTypeAuditPlanCountByProject(projectUid string) ([]*DBTypeAuditPlanCount, error) {
	dBTypeAuditPlanCounts := []*DBTypeAuditPlanCount{}
	err := s.db.Model(AuditPlan{}).
		Select("audit_plans.db_type, audit_plans.type, count(1) audit_plan_count").
		Where("audit_plans.project_id=?", projectUid).
		Group("audit_plans.db_type, audit_plans.type").Scan(&dBTypeAuditPlanCounts).Error
	return dBTypeAuditPlanCounts, errors.New(errors.ConnectStorageError, err)
}
