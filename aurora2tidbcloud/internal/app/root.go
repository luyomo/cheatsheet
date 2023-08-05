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
)

func Run(gOpt configs.Options) error {
	config, err := configs.ReadConfigFile(gOpt.ConfigFile)
	if err != nil {
		return err
	}
	fmt.Printf("The configs are : %#v \n", config)

	// Create CloudFormation
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

	return nil
}
