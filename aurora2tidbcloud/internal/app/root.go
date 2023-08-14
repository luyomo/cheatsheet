package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app/configs"
	cfapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/cloudformation"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/joomcode/errorx"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
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

	cfapi, err := cfapilib.NewCFAPI(nil)
	if err != nil {
		return err
	}

	// ********** ********** Create CloudFormation
	ctx := context.WithValue(context.Background(), "clusterName", "aurora2tidbcloud-ddl") // The name is used for stackname of ddl export
	ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")                       // The clusterType is used for tags

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(config.LambdaVPC.VpcID)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SubnetsIds"), ParameterValue: aws.String(strings.Join(config.LambdaVPC.Subnets, ","))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SecurityGroupIds"), ParameterValue: aws.String(config.LambdaVPC.SecurityGroupID)})

	if err := cfapi.CreateStack("aurora2tidbcloud-ddl", "https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqldump-to-s3.yaml", &parameters, nil); err != nil {
		return err
	}

	// lambdaTask := task.NewBuilder().
	// 	CreateCloudFormationByS3URL("https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqldump-to-s3.yaml", &parameters, &[]types.Tag{
	// 		{Key: aws.String("Type"), Value: aws.String("aurora")},
	// 		{Key: aws.String("Scope"), Value: aws.String("private")},
	// 	}).
	// 	BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	// if err := lambdaTask.Execute(ctxt.New(ctx, 1)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	/* ****************************************************************** */
	// 003. Get S3 ARN created by cloudformation
	/* ****************************************************************** */

	s3arn, err := cfapi.GetStackResource("aurora2tidbcloud-ddl", "S3Bucket")
	if err != nil {
		return err
	}
	fmt.Printf("The s3 bucket are : <%#v> \n\n\n", *s3arn)

	config.BucketInfo.BucketName = *s3arn

	/* ****************************************************************** */
	// 004. Get dumpling lambda function from stack
	/* ****************************************************************** */
	lambdaDDLExport, err := cfapi.GetStackResource("aurora2tidbcloud-ddl", "ddlExport")
	if err != nil {
		return err
	}
	fmt.Printf("lambda function: %#v \n\n\n", *lambdaDDLExport)

	/* ****************************************************************** */
	// 005. Call lambda function to dumpling ddl
	/* ****************************************************************** */
	// Dumpling the ddl
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	fmt.Printf("The configs are : %#v \n", config.SourceDB)
	dbConn, err := json.Marshal(config)
	if err != nil {
		return err
	}
	fmt.Printf("The database : %s \n\n\n", string(dbConn))
	client := lambda.NewFromConfig(cfg)

	fmt.Printf("The db connection: <%#v> \n\n\n", string(dbConn))
	output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: lambdaDDLExport,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        dbConn})
	if err != nil {
		return err
	}
	fmt.Printf("The output on <%s>\n", string(output.Payload))

	/* ****************************************************************** */
	// 002. Create lambda function: binlog position
	/* ****************************************************************** */
	if err := cfapi.CreateStack("aurora2tidbcloud-binlog", "https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqlBinglogInfo.yaml", &parameters, nil); err != nil {
		return err
	}

	/* ****************************************************************** */
	// 005. Get binlog position fetch lambda function from stack
	/* ****************************************************************** */
	// lambdaDDLExport, err := cfapi.GetStackResource("aurora2tidbcloud-binlog", "ddlExport")

	lambdaFetchBinlogPos, err := cfapi.GetStackResource("aurora2tidbcloud-binlog", "ddlExport")
	if err != nil {
		return err
	}

	fmt.Printf("The lambda function are : <%#v> \n\n\n", *lambdaFetchBinlogPos)

	/* ****************************************************************** */
	// 006. Call lambda function to fetch binlog position
	/* ****************************************************************** */

	output, err = client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: lambdaFetchBinlogPos,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        dbConn})
	if err != nil {
		return err
	}
	fmt.Printf("The output: <%s>\n", string(output.Payload))
	var binlogPos []interface{}
	err = json.Unmarshal(output.Payload, &binlogPos)
	if err != nil {
		return err
	}
	fmt.Printf("The binlog pos is <%#v> \n\n\n", binlogPos[0])
	return nil

	/* ****************************************************************** */
	// 002. Create lambda function: binlog position
	/* ****************************************************************** */
	// ctx = context.WithValue(ctx, "clusterName", "aurora2tidbcloud-binlog")
	// // ctx = context.WithValue(ctx, "clusterType", "mysqldump")

	// lambdaTask := task.NewBuilder().
	// 	CreateCloudFormationByS3URL("https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqlBinglogInfo.yaml", &parameters, &[]types.Tag{
	// 		{Key: aws.String("Type"), Value: aws.String("aurora")},
	// 		{Key: aws.String("Scope"), Value: aws.String("private")},
	// 	}).
	// 	BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	// if err := lambdaTask.Execute(ctxt.New(ctx, 1)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	// fmt.Printf("The lambda function are : <%#v> \n\n\n", *lambdaDDLExport)

	// ********** **********

	/* ****************************************************************** */
	// 008. Taken Aurora snapshot
	/* ****************************************************************** */
	auroraDBName := "jay-labmda"
	snapshotARN, err := awsutils.GetSnapshot(auroraDBName)
	if err != nil {
		return err
	}
	fmt.Printf("The snapshot is : <%#v> \n\n\n", *snapshotARN)
	if *snapshotARN == "" {
		// snapshotARN, err = awsutils.RDSSnapshotTaken("lambda-fetch-binlog", "mysql-bin-changelog.000009", 154)
		snapshotARN, err = awsutils.RDSSnapshotTaken(auroraDBName, binlogPos[2].(string), binlogPos[3].(float64))
		if err != nil {
			return err
		}
		// fmt.Printf("created snapshot arn : %s \n\n\n", *snapshotARN)
	}

	/* ****************************************************************** */
	// 009. Export snapshot to s3(parquet)
	/* ****************************************************************** */
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})
	fmt.Printf("Starting to export data to s3 \n\n\n")
	// The original cluster name is [jay-lambda]: The name has to be consistent to snapshot name
	// The cluster name has to be consistent with tidb cloud's cluster name.
	ctx = context.WithValue(ctx, "clusterName", auroraDBName)
	// ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")

	fmt.Printf("The s3 port: <s3://%s/test> \n\n\n", *s3arn)
	// Before make the snapshot export and role make consistenct, separate it.
	// snapshot taken
	export2s3 := task.NewBuilder().
		CreateKMS("s3").
		AuroraSnapshotExportS3(nil, fmt.Sprintf("s3://%s/test", *s3arn), &timer). // 03. Export data from snapshot to S3. -> task 01/02
		BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	if err := export2s3.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err

	}
	return nil

	/* ****************************************************************** */
	// 010. TODO: Run DDL against TiDB Cloud
	/* ****************************************************************** */

	/* ****************************************************************** */
	// 011. TODO: Import data to TiDB Cloud from S3
	/* ****************************************************************** */
	tidbCloudName := "scalingtest"
	tidbProjectID := "1372813089206751438"

	ctx = context.WithValue(ctx, "clusterName", tidbCloudName)
	importtask := task.NewBuilder().
		MakeRole4ExternalAccess(tidbProjectID, fmt.Sprintf("s3://%s/test", *s3arn), &timer). // 04. Make role for TiDB Cloud import -> task 03
		CreateTiDBCloudImport(tidbProjectID, "s3import", &timer).                            // 05. Import data into TiDB Cloud from S3 -> task 04
		BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	if err := importtask.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err

	}

	return nil
}
