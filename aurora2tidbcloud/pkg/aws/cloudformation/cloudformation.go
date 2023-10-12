// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	// "time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awscommon "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws"
)

func MapTag() *map[string]string {
	return &map[string]string{
		"clusterName":    "Name",
		"clusterType":    "Cluster",
		"subClusterType": "Type",
		"scope":          "Scope",
		"component":      "Component",
	}
}

type CloudformationAPI struct {
	client *cloudformation.Client

	mapArgs *map[string]string
}

func NewCFAPI(mapArgs *map[string]string) (*CloudformationAPI, error) {
	cfApi := CloudformationAPI{}

	if mapArgs != nil {
		cfApi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	cfApi.client = cloudformation.NewFromConfig(cfg)

	return &cfApi, nil
}

func (e *CloudformationAPI) DestroyStack(stackName string) error {
	stack, err := e.GetStack(stackName)
	if err != nil {
		return err
	}
	if stack != nil {

		if _, err = e.client.DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{StackName: aws.String(stackName)}); err != nil {
			return err
		}

		if err := awscommon.WaitUntilResouceAvailable(0, 0, 1, func() (bool, error) {
			stack, err := e.GetStack(stackName)
			if err != nil {
				return false, err
			}
			if stack == nil {
				return true, nil
			}
			return false, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (e *CloudformationAPI) GetStackResource(stackName, logicalResourceId string) (*string, error) {
	stack, err := e.GetStack(stackName)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, nil
	}

	describeStackResource, err := e.client.DescribeStackResource(context.TODO(), &cloudformation.DescribeStackResourceInput{StackName: aws.String(stackName), LogicalResourceId: aws.String(logicalResourceId)})
	if err != nil {
		return nil, err
	}
	return describeStackResource.StackResourceDetail.PhysicalResourceId, nil

}

func (e *CloudformationAPI) GetStack(stackName string) (*types.StackSummary, error) {
	resp, err := e.client.ListStacks(context.TODO(), &cloudformation.ListStacksInput{})
	if err != nil {
		return nil, err
	}

	for _, stackSummary := range resp.StackSummaries {
		if *stackSummary.StackName == stackName && stackSummary.StackStatus != types.StackStatusDeleteComplete {
			return &stackSummary, nil
		}
	}
	return nil, nil
}

func (e *CloudformationAPI) CreateStack(stackName, fileUrl string, parameters *[]types.Parameter, tags *[]types.Tag) error {
	stack, err := e.GetStack(stackName)
	if err != nil {
		return err
	}
	if stack == nil {
		parsedS3Dir, err := url.Parse(fileUrl)
		if err != nil {
			return err
		}
		if parsedS3Dir.Scheme == "https" {
			stackInput := &cloudformation.CreateStackInput{
				StackName:    aws.String(stackName),
				TemplateURL:  aws.String(fileUrl),
				Capabilities: []types.Capability{types.CapabilityCapabilityIam},
			}
			if parameters != nil {
				stackInput.Parameters = *parameters
			}
			if tags != nil {
				stackInput.Tags = *tags
			}

			if _, err = e.client.CreateStack(context.TODO(), stackInput); err != nil {
				return err
			}
		} else {
			content, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", parsedS3Dir.Host, parsedS3Dir.Path))
			if err != nil {
				return err
			}
			templateBody := string(content)

			stackInput := &cloudformation.CreateStackInput{
				StackName:    aws.String(stackName),
				TemplateBody: aws.String(templateBody),
				Capabilities: []types.Capability{types.CapabilityCapabilityIam},
				Parameters:   *parameters,
				Tags:         *tags,
			}

			if _, err = e.client.CreateStack(context.TODO(), stackInput); err != nil {
				return err
			}
		}

		if err := awscommon.WaitUntilResouceAvailable(0, 0, 1, func() (bool, error) {
			stack, err := e.GetStack(stackName)
			if err != nil {
				return false, err
			}
			if stack == nil {
				return false, errors.New(fmt.Sprintf("No stack created[%s]", stackName))
			}
			if stack.StackStatus == types.StackStatusCreateComplete {
				return true, nil
			}
			return false, nil
		}); err != nil {
			return err
		}

	}
	return nil
}

// func waitUntilResouceAvailable(_interval, _timeout time.Duration, expectNum int, _readResource func() (bool, error)) error {
// 	if _interval == 0 {
// 		_interval = 60 * time.Second
// 	}

// 	if _timeout == 0 {
// 		_timeout = 60 * time.Minute
// 	}

// 	timeout := time.After(_timeout)
// 	d := time.NewTicker(_interval)

// 	for {
// 		// Select statement
// 		select {
// 		case <-timeout:
// 			return errors.New("Timed out")
// 		case _ = <-d.C:
// 			isFinished, err := _readResource()
// 			if err != nil {
// 				return err
// 			}

// 			if isFinished == true {
// 				return nil
// 			}
// 		}
// 	}
// }
