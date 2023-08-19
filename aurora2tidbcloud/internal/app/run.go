package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app/configs"
	cfapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/cloudformation"
	iamapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/iam"
	kmsapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/kms"
	rdsapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/rds"
	"github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/tidbcloud"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go/ptr"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// Summary: Migrate the data from Aurora to TiDB Cloud
// 001. Create lambda function: dumpling
// 002. Create lambda function: binlog position
func Run(gOpt configs.Options) error {
	/* ****************************************************************** */
	// 001. Config read
	/* ****************************************************************** */
	config, err := configs.ReadConfigFile(gOpt.ConfigFile)
	if err != nil {
		return err
	}
	fmt.Printf("The configs are : %#v \n", config)

	/* ****************************************************************** */
	// 002. Get binlog position
	/* ****************************************************************** */
	binlogFile, binlogPos, err := fetchBinlogInfo(config)
	if err != nil {
		return err
	}
	fmt.Printf("binlog file: %s, position: %d \n\n\n", *binlogFile, *binlogPos)

	/* ****************************************************************** */
	// 002. Backup DDL
	/* ****************************************************************** */
	s3arn, err := exportDDL(config)
	if err != nil {
		return err
	}

	kmsArn, err := getKMSArn()
	if err != nil {
		return err
	}

	_, err = dataExport(&config.SourceDB.AuroraClusterName, binlogFile, binlogPos, s3arn, &config.BucketInfo.S3Key, kmsArn)
	if err != nil {
		return err
	}

	if err := importData2TiDBCloud(s3arn, &config.BucketInfo.S3Key, kmsArn, &config.TiDBCloud); err != nil {
		return err
	}

	return nil
}

// Input:
// Output:
//
//	Export DDL to S3
//
// 01. Create stack
// 02. Get lambda function
// 03. Get S3 Bucket
// 04. Make a call to run the dumpling
func exportDDL(configs *configs.Config) (*string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := lambda.NewFromConfig(cfg)

	cfapi, err := cfapilib.NewCFAPI(nil)
	if err != nil {
		return nil, err
	}

	// ctx := context.WithValue(context.Background(), "clusterName", "aurora2tidbcloud-ddl") // The name is used for stackname of ddl export
	// ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")                       // The clusterType is used for tags

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(configs.LambdaVPC.VpcID)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SubnetsIds"), ParameterValue: aws.String(strings.Join(configs.LambdaVPC.Subnets, ","))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SecurityGroupIds"), ParameterValue: aws.String(configs.LambdaVPC.SecurityGroupID)})

	if err := cfapi.CreateStack(STACKNAME_DUMPLING, LAMBDA_FUNCTION_DDL, &parameters, nil); err != nil {
		return nil, err
	}

	/* ****************************************************************** */
	// 004. Get dumpling lambda function from stack
	/* ****************************************************************** */
	lambdaDDLExport, err := cfapi.GetStackResource(STACKNAME_DUMPLING, "ddlExport") // Cloud formation logical name -> lambda function name
	if err != nil {
		return nil, err
	}
	fmt.Printf("lambda function: %#v \n\n\n", *lambdaDDLExport)

	/* ****************************************************************** */
	// 003. Get S3 ARN created by cloudformation
	/* ****************************************************************** */

	s3arn, err := cfapi.GetStackResource(STACKNAME_DUMPLING, "S3Bucket") // Cloud formation logical name -> S3 Arn
	if err != nil {
		return nil, err
	}
	fmt.Printf("The s3 bucket are : <%#v> \n\n\n", *s3arn)

	configs.BucketInfo.BucketName = *s3arn

	theConfig := *configs
	theConfig.BucketInfo.S3Key = configs.BucketInfo.S3Key + "/ddl"
	dbConn, err := json.Marshal(theConfig)
	if err != nil {
		return nil, err
	}

	fmt.Printf("The db connection: <%#v> \n\n\n", string(dbConn))
	output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: lambdaDDLExport,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        dbConn})
	if err != nil {
		return nil, err
	}
	fmt.Printf("The output on <%s>\n", string(output.Payload))

	return s3arn, nil
}

func fetchBinlogInfo(configs *configs.Config) (*string, *int64, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, nil, err
	}

	client := lambda.NewFromConfig(cfg)

	cfapi, err := cfapilib.NewCFAPI(nil)
	if err != nil {
		return nil, nil, err
	}

	dbConn, err := json.Marshal(configs)
	if err != nil {
		return nil, nil, err
	}

	// ctx := context.WithValue(context.Background(), "clusterName", "aurora2tidbcloud-ddl") // The name is used for stackname of ddl export
	// ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")                       // The clusterType is used for tags

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(configs.LambdaVPC.VpcID)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SubnetsIds"), ParameterValue: aws.String(strings.Join(configs.LambdaVPC.Subnets, ","))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SecurityGroupIds"), ParameterValue: aws.String(configs.LambdaVPC.SecurityGroupID)})

	/* ****************************************************************** */
	// 002. Create lambda function: binlog position
	/* ****************************************************************** */
	if err := cfapi.CreateStack(STACKNAME_BINLOG, LAMBDA_FUNCTION_BINLOG, &parameters, nil); err != nil {
		return nil, nil, err
	}

	/* ****************************************************************** */
	// 005. Get binlog position fetch lambda function from stack
	/* ****************************************************************** */
	// lambdaDDLExport, err := cfapi.GetStackResource("aurora2tidbcloud-binlog", "ddlExport")

	lambdaFetchBinlogPos, err := cfapi.GetStackResource(STACKNAME_BINLOG, "ddlExport") // Cloudformation logical name -> lambda function name
	if err != nil {
		return nil, nil, err
	}

	fmt.Printf("The lambda function are : <%#v> \n\n\n", *lambdaFetchBinlogPos)

	/* ****************************************************************** */
	// 006. Call lambda function to fetch binlog position
	/* ****************************************************************** */

	output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: lambdaFetchBinlogPos,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        dbConn})
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("The output: <%s>\n", string(output.Payload))
	var binlogPos []interface{}
	err = json.Unmarshal(output.Payload, &binlogPos)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("The binlog pos is <%#v> \n\n\n", binlogPos[0])
	return aws.String(binlogPos[2].(string)), aws.Int64(int64(binlogPos[3].(float64))), nil
}

func dataExport(auroraClusterName, binlogFile *string, binlogPos *int64, s3arn, s3key, kmsArn *string) (*string, error) {
	/* ****************************************************************** */
	// 003. snapshot taken
	/* ****************************************************************** */
	rdsapi, err := rdsapilib.NewRdsAPI(nil)
	if err != nil {
		return nil, err
	}

	snapshotArn, err := rdsapi.RDSSnapshotTaken(*auroraClusterName, *binlogFile, *binlogPos)
	if err != nil {
		return nil, err
	}
	fmt.Printf("The snap: <%#v> \n\n\n", *snapshotArn)

	/* ****************************************************************** */
	// 004. Role for export preparation
	/* ****************************************************************** */

	iamapi, err := iamapilib.NewIAMAPI(nil)
	if err != nil {
		return nil, err
	}

	roleArn, err := iamapi.CreateRole4S3ByRDS(EXPORT_ROLE, aws.String(MODULE_NAME), fmt.Sprintf("s3://%s/%s", *s3arn, *s3key), kmsArn, nil)
	if err != nil {
		return nil, err
	}

	/* ****************************************************************** */
	// 006. Data export to S3
	/* ****************************************************************** */
	exportTask, err := rdsapi.ExportSnapshot2S3(MODULE_NAME, snapshotArn, kmsArn, roleArn, fmt.Sprintf("s3://%s/%s/data", *s3arn, *s3key))
	if err != nil {
		return nil, err
	}
	fmt.Printf("The export task is <%#v> \n\n\n", exportTask)

	return exportTask.S3Prefix, nil
}

func getKMSArn() (*string, error) {
	/* ****************************************************************** */
	// 005. KMS preparation
	/* ****************************************************************** */
	// 01. Create KMS if it does not exist
	kmsapi, err := kmsapilib.NewKmsAPI(nil)
	if err != nil {
		return nil, err
	}
	kmsArn, err := kmsapi.GetKMSKeyByName("jay-labmda-aurora2tidbcloud")
	if err != nil {
		return nil, err
	}
	fmt.Printf("The mks arn is : <%s> \n\n\n", *kmsArn)
	return kmsArn, nil
}

func importData2TiDBCloud(s3arn, s3prefix, kmsArn *string, tidbCloud *configs.TiDBCloud) error {
	/* ****************************************************************** */
	// 006. Role for import preparation
	/* ****************************************************************** */
	tidbcloudApi, err := tidbcloud.NewTiDBCloudAPI(string(tidbCloud.ProjectID), tidbCloud.ClusterName, nil)
	if err != nil {
		return err
	}
	accountId, externalId, err := tidbcloudApi.GetImportTaskRoleInfo()
	if err != nil {
		return err
	}
	fmt.Printf("account id: %s  externalId: %s \n\n\n", *accountId, *externalId)

	iamapi, err := iamapilib.NewIAMAPI(nil)
	if err != nil {
		return err
	}

	importRoleArn, err := iamapi.CreateRole4S3External(IMPORT_ROLE, aws.String(MODULE_NAME), kmsArn, accountId, externalId, fmt.Sprintf("s3://%s/", *s3arn), nil)
	if err != nil {
		return err
	}
	fmt.Printf("The import role is: %s \n\n\n", *importRoleArn)
	fmt.Printf("The prefix is: %s \n\n\n", *s3prefix)

	/* ****************************************************************** */
	// 007. Schema ddl creation
	/* ****************************************************************** */
	if err := tidbcloudApi.StartImportTask(ptr.String(tidbCloud.ClusterName), ptr.String(fmt.Sprintf("s3://%s/%s/ddl", *s3arn, *s3prefix)), importRoleArn, ptr.String("SQL")); err != nil {
		return err
	}
	// s3: //aurora2tidbcloud-ddl-data/lambda/data/

	/* ****************************************************************** */
	// 008. Data import
	/* ****************************************************************** */
	fmt.Printf("Completed to generate the DDL \n\n\n")
	// s3://aurora2tidbcloud-ddl-data/test/arrora2tidbcloud-20230816112919/
	if err := tidbcloudApi.StartImportTask(ptr.String(tidbCloud.ClusterName), ptr.String(fmt.Sprintf("s3://%s/%s/data", *s3arn, *s3prefix)), importRoleArn, ptr.String("AURORA_SNAPSHOT")); err != nil {
		return err
	}
	return nil
}
