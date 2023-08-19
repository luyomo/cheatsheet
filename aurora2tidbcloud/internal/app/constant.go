package app

import (
// "context"
// "encoding/json"
// "fmt"
// "log"
// "strings"
// "github.com/aws/aws-sdk-go-v2/aws"
// "github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app/configs"
// cfapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/cloudformation"
// iamapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/iam"
// s3apilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/s3"
)

const (
	MODULE_NAME            = "aurora2tidbcloud"              // Used to determinte the aws resource
	IMPORT_ROLE            = "aurora2tidbcloud-import-role"  // Role used by tidb cloud openapi to access s3
	EXPORT_ROLE            = "aurora2tidbcloud-export-role"  // Role used to export data from aurora snapshot to s3
	STACKNAME_BINLOG       = "aurora2tidbcloud-binlog"       // Stack name for lambda to fetch binlog info
	STACKNAME_DUMPLING     = "aurora2tidbcloud-ddl-dumpling" // Stack name for lambda to dump ddl to s3 by dumpling
	LAMBDA_FUNCTION_DDL    = "https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqldump-to-s3.yaml"
	LAMBDA_FUNCTION_BINLOG = "https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqlBinglogInfo.yaml"
)
