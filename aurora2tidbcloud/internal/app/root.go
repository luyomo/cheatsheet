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

func Run(gOpt configs.Options) error {
	/* ****************************************************************** */
	/* ****************************************************************** */
	config, err := configs.ReadConfigFile(gOpt.ConfigFile)
	if err != nil {
		return err
	}
	fmt.Printf("The configs are : %#v \n", config)

	// ********** ********** Create CloudFormation
	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(config.LambdaVPC.VpcID)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SubnetsIds"), ParameterValue: aws.String(strings.Join(config.LambdaVPC.Subnets, ","))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("SecurityGroupIds"), ParameterValue: aws.String(config.LambdaVPC.SecurityGroupID)})

	ctx := context.WithValue(context.Background(), "clusterName", "mysqldump-to-s3")
	ctx = context.WithValue(ctx, "clusterType", "binlogPos")

	lambdaTask := task.NewBuilder().
		CreateCloudFormationByS3URL("https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqldump-to-s3.yaml", &parameters, &[]types.Tag{
			{Key: aws.String("Type"), Value: aws.String("aurora")},
			{Key: aws.String("Scope"), Value: aws.String("private")},
		}).
		BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	if err := lambdaTask.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	// ********** ********** Create CloudFormation
	ctx = context.WithValue(context.Background(), "clusterName", "jay-lambda")
	ctx = context.WithValue(ctx, "clusterType", "mysqldump")

	lambdaTask = task.NewBuilder().
		CreateCloudFormationByS3URL("https://jay-data.s3.amazonaws.com/lambda/cloudformation/mysqlBinglogInfo.yaml", &parameters, &[]types.Tag{
			{Key: aws.String("Type"), Value: aws.String("aurora")},
			{Key: aws.String("Scope"), Value: aws.String("private")},
		}).
		BuildAsStep(fmt.Sprintf("  - Preparing lambda service ... ..."))

	if err := lambdaTask.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	cfapi, err := cfapilib.NewCFAPI(nil)
	if err != nil {
		return err
	}

	s3arn, err := cfapi.GetStackResource("mysqldump-to-s3", "S3Bucket")
	if err != nil {
		return err
	}
	fmt.Printf("The lambda function are : <%#v> \n\n\n", *s3arn)

	config.BucketInfo.BucketName = *s3arn

	lambdaDDLExport, err := cfapi.GetStackResource("mysqldump-to-s3", "ddlExport")
	if err != nil {
		return err
	}

	fmt.Printf("The lambda function are : <%#v> \n\n\n", *lambdaDDLExport)

	// lambdaFetchBinlogPos, err := cfapi.GetStackResource("lambda-fetch-binlog", "ddlExport")
	lambdaFetchBinlogPos, err := cfapi.GetStackResource("jay-lambda", "ddlExport")
	if err != nil {
		return err
	}

	fmt.Printf("The lambda function are : <%#v> \n\n\n", *lambdaFetchBinlogPos)

	// ********** **********
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

	output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: lambdaFetchBinlogPos,
		// InvocationType: lambdatypes.InvocationTypeEvent,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		// Payload:        []byte("{\"RDSConn\": {\"rds_host\":\"localhost\",\"rds_port\":3306,\"rds_user\":\"admin\",\"rds_password\":\"1234Abcd\"}}")})
		Payload: dbConn})
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

	// Dumpling the ddl
	fmt.Printf("The db connection: <%#v> \n\n\n", string(dbConn))
	output, err = client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: lambdaDDLExport,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		// Payload:        []byte(fmt.Sprintf("{\"RDSConn\": {\"rds_host\":\"localhost\",\"rds_port\":3306,\"rds_user\":\"admin\",\"rds_password\":\"1234Abcd\"}, \"S3\":{\"BucketName\":\"%s\", \"S3Key\":\"lambda/data\"}}", *s3arn))})
		Payload: dbConn})
	if err != nil {
		return err
	}
	fmt.Printf("The output on <%s>\n", string(output.Payload))

	snapshotARN, err := awsutils.GetSnapshot("jay-labmda")
	if err != nil {
		return err
	}
	fmt.Printf("The snapshot is : <%#v> \n\n\n", *snapshotARN)
	if *snapshotARN == "" {
		// snapshotARN, err = awsutils.RDSSnapshotTaken("lambda-fetch-binlog", "mysql-bin-changelog.000009", 154)
		snapshotARN, err = awsutils.RDSSnapshotTaken("jay-labmda", binlogPos[2].(string), binlogPos[3].(float64))
		if err != nil {
			return err
		}
		// fmt.Printf("created snapshot arn : %s \n\n\n", *snapshotARN)
	}

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})
	fmt.Printf("Starting to export data to s3 \n\n\n")
	ctx = context.WithValue(ctx, "clusterName", "jay-labmda")
	ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")

	fmt.Printf("The s3 port: <%s>\n\n\n", *s3arn)
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
}
