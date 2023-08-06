package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app/configs"

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

	ctx := context.WithValue(context.Background(), "clusterName", "lambda-fetch-binlog")
	ctx = context.WithValue(ctx, "clusterType", "binlogPos")

	lambdaTask := task.NewBuilder().
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

	// ********** **********
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := lambda.NewFromConfig(cfg)

	output, err := client.Invoke(context.TODO(), &lambda.InvokeInput{FunctionName: aws.String("lambda-fetch-binlog-ddlExport-SNPdBVNH2skJ"),
		// InvocationType: lambdatypes.InvocationTypeEvent,
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        []byte("{\"RDSConn\": {\"rds_host\":\"jay-labmda.cluster-cxmxisy1o2a2.us-east-1.rds.amazonaws.com\",\"rds_port\":3306,\"rds_user\":\"admin\",\"rds_password\":\"1234Abcd\"}}")})
	if err != nil {
		return err
	}
	fmt.Printf("The output: <%s>\n", (output.Payload))
	snapshotARN, err := awsutils.GetSnapshot("jay-labmda")
	if err != nil {
		return err
	}
	fmt.Printf("The snapshot is : <%#v> \n\n\n", *snapshotARN)
	if *snapshotARN == "" {
		snapshotARN, err = awsutils.RDSSnapshotTaken("lambda-fetch-binlog", "mysql-bin-changelog.000009", 154)
		if err != nil {
			return err
		}
		// fmt.Printf("created snapshot arn : %s \n\n\n", *snapshotARN)
	}

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	ctx = context.WithValue(ctx, "clusterName", "jay-labmda")
	ctx = context.WithValue(ctx, "clusterType", "aurora2tidbcloud")

	export2s3 := task.NewBuilder().
		CreateKMS("s3").
		AuroraSnapshotExportS3(nil, "s3://jay-data/test", &timer). // 03. Export data from snapshot to S3. -> task 01/02
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
