package driver

import (
	"context"
	"database/sql/driver"

	"github.com/actiontech/dms/pkg/dms-common/i18nPkg"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"

	"github.com/sirupsen/logrus"
)

type Plugin interface {
	Close(ctx context.Context)

	// Parse parse sqlText to Node array. sqlText may be single SQL or batch SQLs.
	Parse(ctx context.Context, sqlText string) ([]driverV2.Node, error)

	// Audit sql with rules. sql is single SQL text or multi audit.
	Audit(ctx context.Context, sqls []string) ([]*driverV2.AuditResults, error)

	// GenRollbackSQL generate sql's rollback SQL.
	GenRollbackSQL(ctx context.Context, sql string) (string, i18nPkg.I18nStr, error)

	Ping(ctx context.Context) error
	Exec(ctx context.Context, query string) (driver.Result, error)
	ExecBatch(ctx context.Context, sqls ...string) ([]driver.Result, error)
	Tx(ctx context.Context, queries ...string) ([]driver.Result, error)
	Query(ctx context.Context, sql string, conf *driverV2.QueryConf) (*driverV2.QueryResult, error)
	Explain(ctx context.Context, conf *driverV2.ExplainConf) (*driverV2.ExplainResult, error)
	ExplainJSONFormat(ctx context.Context, conf *driverV2.ExplainConf) (*driverV2.ExplainJSONResult, error)

	// KillProcess uses a new connection to send the "Kill process_id" command to terminate the thread that is currently running.
	KillProcess(ctx context.Context) (err error)

	// Schemas export all supported schemas.
	//
	// For example, performance_schema/performance_schema... which in MySQL is not allowed for auditing.
	Schemas(ctx context.Context) ([]string, error)

	// in v2, this is a virtual api, it is a combination of [ExtractTableFromSQL, GetTableMeta]
	GetTableMetaBySQL(ctx context.Context, conf *GetTableMetaBySQLConf) (*GetTableMetaBySQLResult, error)

	// Introduced from v2.2304.0
	EstimateSQLAffectRows(ctx context.Context, sql string) (*driverV2.EstimatedAffectRows, error)

	GetDatabaseObjectDDL(ctx context.Context, objInfos []*driverV2.DatabaseSchemaInfo) ([]*driverV2.DatabaseSchemaObjectResult, error)

	GetDatabaseDiffModifySQL(ctx context.Context, calibratedDSN *driverV2.DSN, objInfos []*driverV2.DatabasCompareSchemaInfo) ([]*driverV2.DatabaseDiffModifySQLResult, error)

	Backup(ctx context.Context, backupStrategy string, sql string, backupMaxRows uint64) (backupSqls []string, executeResult string, err error)

	RecommendBackupStrategy(ctx context.Context, sql string) (*RecommendBackupStrategyRes, error)
}

type RecommendBackupStrategyRes struct {
	BackupStrategy    string
	BackupStrategyTip string
	TablesRefer       []string
	SchemasRefer      []string
}

type PluginProcessor interface {
	GetDriverMetas() (*driverV2.DriverMetas, error)
	Open(*logrus.Entry, *driverV2.Config) (Plugin, error)
	Stop() error
}

type GetTableMetaBySQLConf struct {
	// this SQL should be a single SQL
	Sql string
}

type GetTableMetaBySQLResult struct {
	TableMetas []*TableMeta
}

type TableMeta struct {
	driverV2.Table
	driverV2.TableMeta
}
