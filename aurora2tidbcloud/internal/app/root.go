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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	// "github.com/joomcode/errorx"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/task"
	// "github.com/luyomo/OhMyTiUP/pkg/ctxt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	// awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
)

// Summary: Migrate the data from Aurora to TiDB Cloud
// 001. Create lambda function: dumpling
// 002. Create lambda function: binlog position
func Run(gOpt configs.Options) error {
	/* ****************************************************************** */
	// 001. Create lambda function: dumpling
	/* ****************************************************************** */
	config, err := configs.ReadConfigFile(gOpt.ConfigFile)
	if err != nil {
		return err
	}
	fmt.Printf("The configs are : %#v \n", config)

	s3arn, err := exportDDL(config)
	if err != nil {
		return err
	}

	binlogFile, binlogPos, err := fetchBinlogInfo(config)
	if err != nil {
		return err
	}
	fmt.Printf("binlog file: %s, position: %d \n\n\n", *binlogFile, *binlogPos)

	/* ****************************************************************** */
	// 008. Taken Aurora snapshot
	/* ****************************************************************** */
	rdsapi, err := rdsapilib.NewRdsAPI(nil)
	if err != nil {
		return err
	}

	auroraDBName := "jay-labmda"

	// snapshot, err := rdsapi.GetSnapshotByBinlog(auroraDBName, *binlogFile, *binlogPos)
	// if err != nil {
	// 	return err
	// }

	snapshotArn, err := rdsapi.RDSSnapshotTaken(auroraDBName, *binlogFile, *binlogPos)
	if err != nil {
		return err
	}
	fmt.Printf("The snap: <%#v> \n\n\n", *snapshotArn)

	iamapi, err := iamapilib.NewIAMAPI(nil)
	if err != nil {
		return err
	}
	iamRoleName := "jay-test-role"
	roleArn, err := iamapi.CreateRole4S3ByRDS(iamRoleName, aws.String("aurora2tidbcloud"), fmt.Sprintf("s3://%s/test", *s3arn), nil)
	if err != nil {
		return err
	}

	/* ****************************************************************** */
	// 009. Export snapshot to s3(parquet)
	/* ****************************************************************** */
	// 01. Create KMS if it does not exist
	kmsapi, err := kmsapilib.NewKmsAPI(nil)
	if err != nil {
		return err
	}
	kmsarn, err := kmsapi.GetKMSKeyByName("jay-labmda-aurora2tidbcloud")
	if err != nil {
		return err
	}
	fmt.Printf("The mks arn is : <%s> \n\n\n", *kmsarn)

	// 04. Export data to s3

	// snapshotArn, err = rdsapi.GetLatestSnapshot("jay-labmda")
	// if err != nil {
	// 	return err
	// }

	exportTask, err := rdsapi.ExportSnapshot2S3("arrora2tidbcloud", snapshotArn, kmsarn, roleArn, fmt.Sprintf("s3://%s/test", *s3arn))
	if err != nil {
		return err
	}
	fmt.Printf("The export task is <%#v> \n\n\n", exportTask)

	return nil
	// // 02. Create policy if it does not exist
	// // 03. Create role if it does not exist
	// iamapi, err = iamapilib.NewIAMAPI(nil)
	// if err != nil {
	// 	return err
	// }
	// iamRoleName = "jay-test-role"
	// roleArn, err = iamapi.CreateRole4S3ByRDS(iamRoleName, aws.String("aurora2tidbcloud"), fmt.Sprintf("s3://%s/test", *s3arn), nil)
	// if err != nil {
	// 	return err
	// }

	// // 04. Export data to s3

	// snapshotArn, err = rdsapi.GetLatestSnapshot("jay-labmda")
	// if err != nil {
	// 	return err
	// }

	// exportTask, err := rdsapi.GetLatestExportTaskBySnapshot(snapshotArn)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("The export task: <%#v> \n\n\n", *exportTask)

	// if exportTask == nil {
	// exportTask, err = rdsapi.ExportSnapshot2S3("arrora2tidbcloud", snapshotArn, kmsarn, roleArn, fmt.Sprintf("s3://%s/data01", *s3arn))
	// if err != nil {
	// 	return err
	// }
	// // }

	// fmt.Printf("The export task is <%#v> \n\n\n", exportTask)

	// return nil

	/* ****************************************************************** */
	// 009. Export snapshot to s3(parquet)
	/* ****************************************************************** */
	// var timer awsutils.ExecutionTimer
	// timer.Initialize([]string{"Step", "Duration(s)"})
	// fmt.Printf("Starting to export data to s3 \n\n\n")
	// // The original cluster name is [jay-lambda]: The name has to be consistent to snapshot name
	// // The cluster name has to be consistent with tidb cloud's cluster name.
	// ctx = context.WithValue(ctx, "clusterName", auroraDBName)
	// // ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")

	// fmt.Printf("The s3 port: <s3://%s/test> \n\n\n", *s3arn)
	// // Before make the snapshot export and role make consistenct, separate it.
	// // snapshot taken
	// export2s3 := task.NewBuilder().
	// 	CreateKMS("s3").
	// 	AuroraSnapshotExportS3(nil, fmt.Sprintf("s3://%s/test", *s3arn), &timer). // 03. Export data from snapshot to S3. -> task 01/02
	// 	BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	// if err := export2s3.Execute(ctxt.New(ctx, 1)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err

	// }
	// return nil

	/* ****************************************************************** */
	// 010. TODO: Run DDL against TiDB Cloud
	/* ****************************************************************** */

	/* ****************************************************************** */
	// 011. TODO: Import data to TiDB Cloud from S3
	/* ****************************************************************** */
	// tidbCloudName := "scalingtest"
	// tidbProjectID := "1372813089206751438"

	// ctx = context.WithValue(ctx, "clusterName", tidbCloudName)
	// importtask := task.NewBuilder().
	// 	MakeRole4ExternalAccess(tidbProjectID, fmt.Sprintf("s3://%s/test", *s3arn), &timer). // 04. Make role for TiDB Cloud import -> task 03
	// 	CreateTiDBCloudImport(tidbProjectID, "s3import", &timer).                            // 05. Import data into TiDB Cloud from S3 -> task 04
	// 	BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	// if err := importtask.Execute(ctxt.New(ctx, 1)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err

	// }

	// return nil
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

	ctx := context.WithValue(context.Background(), "clusterName", "aurora2tidbcloud-ddl") // The name is used for stackname of ddl export
	ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")                       // The clusterType is used for tags

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(configs.LambdaVPC.VpcID)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SubnetsIds"), ParameterValue: aws.String(strings.Join(configs.LambdaVPC.Subnets, ","))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SecurityGroupIds"), ParameterValue: aws.String(configs.LambdaVPC.SecurityGroupID)})

	if err := cfapi.CreateStack("aurora2tidbcloud-ddl", "https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqldump-to-s3.yaml", &parameters, nil); err != nil {
		return nil, err
	}

	/* ****************************************************************** */
	// 004. Get dumpling lambda function from stack
	/* ****************************************************************** */
	lambdaDDLExport, err := cfapi.GetStackResource("aurora2tidbcloud-ddl", "ddlExport")
	if err != nil {
		return nil, err
	}
	fmt.Printf("lambda function: %#v \n\n\n", *lambdaDDLExport)

	/* ****************************************************************** */
	// 003. Get S3 ARN created by cloudformation
	/* ****************************************************************** */

	s3arn, err := cfapi.GetStackResource("aurora2tidbcloud-ddl", "S3Bucket")
	if err != nil {
		return nil, err
	}
	fmt.Printf("The s3 bucket are : <%#v> \n\n\n", *s3arn)

	configs.BucketInfo.BucketName = *s3arn
	dbConn, err := json.Marshal(configs)
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

	ctx := context.WithValue(context.Background(), "clusterName", "aurora2tidbcloud-ddl") // The name is used for stackname of ddl export
	ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")                       // The clusterType is used for tags

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(configs.LambdaVPC.VpcID)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SubnetsIds"), ParameterValue: aws.String(strings.Join(configs.LambdaVPC.Subnets, ","))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SecurityGroupIds"), ParameterValue: aws.String(configs.LambdaVPC.SecurityGroupID)})

	/* ****************************************************************** */
	// 002. Create lambda function: binlog position
	/* ****************************************************************** */
	if err := cfapi.CreateStack("aurora2tidbcloud-binlog", "https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqlBinglogInfo.yaml", &parameters, nil); err != nil {
		return nil, nil, err
	}

	/* ****************************************************************** */
	// 005. Get binlog position fetch lambda function from stack
	/* ****************************************************************** */
	// lambdaDDLExport, err := cfapi.GetStackResource("aurora2tidbcloud-binlog", "ddlExport")

	lambdaFetchBinlogPos, err := cfapi.GetStackResource("aurora2tidbcloud-binlog", "ddlExport")
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
