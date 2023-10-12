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

package tidbcloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/smithy-go/ptr"
	awscommon "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws"
	"github.com/luyomo/tidbcloud-sdk-go-v1/pkg/tidbcloud"
)

type TiDBCloudAPI struct {
	client      *tidbcloud.ClientWithResponses
	projectId   string
	clusterId   string
	clusterName string

	mapArgs *map[string]string
}

func NewTiDBCloudAPI(projectId, clusterName string, mapArgs *map[string]string) (*TiDBCloudAPI, error) {
	tidbCloudApi := TiDBCloudAPI{
		projectId:   projectId,
		clusterName: clusterName,
	}

	if mapArgs != nil {
		tidbCloudApi.mapArgs = mapArgs
	}

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return nil, err
	}

	tidbCloudApi.client = client
	if err := tidbCloudApi.setClusterId(); err != nil {
		return nil, err
	}

	return &tidbCloudApi, nil
}

func (t *TiDBCloudAPI) setClusterId() error {
	clusterId, err := t.GetClusterId()
	if err != nil {
		return err
	}
	if clusterId != nil {
		t.clusterId = *clusterId
	}
	return nil
}

func (t *TiDBCloudAPI) GetClusterId() (*string, error) {
	fmt.Printf("The project id is: %s \n\n\n", t.projectId)
	response, err := t.client.ListClustersOfProjectWithResponse(context.Background(), t.projectId, &tidbcloud.ListClustersOfProjectParams{})
	if err != nil {
		return nil, err
	}

	statusCode := response.StatusCode()
	fmt.Printf("The response is : %#v \n\n\n", statusCode)
	switch statusCode {
	case 200:
	case 400:
		return nil, errors.New(fmt.Sprintf("Failed to import data<400>: %s, detail:%#v", *response.JSON400.Message, *response.JSON400.Details))
	case 403:
		return nil, errors.New(fmt.Sprintf("Failed to import data<403>: %s, detail:%#v", *response.JSON403.Message, *response.JSON403.Details))
	case 404:
		fmt.Printf("404 error: %#v \n\n\n", *response.JSON404)
		return nil, errors.New(fmt.Sprintf("Failed to import data<404>: %s, detail:%#v", *response.JSON404.Message, *response.JSON404.Details))
	case 429:
		return nil, errors.New(fmt.Sprintf("Failed to import data<429>: %s, detail:%#v", *response.JSON429.Message, *response.JSON429.Details))
	case 500:
		return nil, errors.New(fmt.Sprintf("Failed to import data<500>: %s, detail:%#v", *response.JSON500.Message, *response.JSON500.Details))
	default:
		return nil, errors.New(fmt.Sprintf("Failed to import data<%d>: %s", statusCode, *response))
	}

	for _, item := range response.JSON200.Items {
		fmt.Printf("Tidb name: %s vs %s vs %s \n\n\n", *item.Name, t.clusterName, (*item.Status.ClusterStatus).(string))
		clusterStatus := (*item.Status.ClusterStatus).(string)
		if *item.Name == t.clusterName && (clusterStatus == "AVAILABLE" || clusterStatus == "IMPORTING") {
			return &item.Id, nil
		}
	}
	return nil, nil
}

func (t *TiDBCloudAPI) getClusterInfo() (*string, *string, error) {
	response, err := t.client.ListClustersOfProjectWithResponse(context.Background(), t.projectId, &tidbcloud.ListClustersOfProjectParams{})
	if err != nil {
		return nil, nil, err
	}

	for _, item := range response.JSON200.Items {
		if *item.Name == t.clusterName {
			return &item.Id, ptr.String((*item.Status.ClusterStatus).(string)), nil
		}
	}
	return nil, nil, nil
}

// Type: AURORA_SNAPSHOT/SQL
func (t *TiDBCloudAPI) StartImportTask(tidbName, s3Dir, importRoleArn, dataSourceType *string) error {
	clusterId, err := t.GetClusterId()
	if err != nil {
		return err
	}

	// Search for the valid S3 backup to import.
	// exportTasks, err := awsutils.GetValidBackupS3(c.clusterName)
	// if err != nil {
	// 	return err
	// }

	var createImportTaskJSONRequestBody tidbcloud.CreateImportTaskJSONRequestBody
	createImportTaskJSONRequestBody.Name = tidbName
	createImportTaskJSONRequestBody.Spec.Source.Type = "S3"
	createImportTaskJSONRequestBody.Spec.Source.Format.Type = *dataSourceType
	createImportTaskJSONRequestBody.Spec.Source.Uri = *s3Dir
	createImportTaskJSONRequestBody.Spec.Source.AwsAssumeRoleAccess = &struct {
		AssumeRole string `json:"assume_role"`
	}{*importRoleArn}

	resImport, err := t.client.CreateImportTaskWithResponse(context.Background(), t.projectId, *clusterId, createImportTaskJSONRequestBody)
	if err != nil {
		return err
	}

	statusCode := resImport.StatusCode()
	switch statusCode {
	case 200:
	case 400:
		return errors.New(fmt.Sprintf("Failed to import data<400>: %s, detail:%#v", *resImport.JSON400.Message, *resImport.JSON400.Details))
	case 403:
		return errors.New(fmt.Sprintf("Failed to import data<403>: %s, detail:%#v", *resImport.JSON403.Message, *resImport.JSON403.Details))
	case 404:
		return errors.New(fmt.Sprintf("Failed to import data<404>: %s, detail:%#v", *resImport.JSON404.Message, *resImport.JSON404.Details))
	case 429:
		return errors.New(fmt.Sprintf("Failed to import data<429>: %s, detail:%#v", *resImport.JSON429.Message, *resImport.JSON429.Details))
	case 500:
		return errors.New(fmt.Sprintf("Failed to import data<500>: %s, detail:%#v", *resImport.JSON500.Message, *resImport.JSON500.Details))
	default:
		return errors.New(fmt.Sprintf("Failed to import data<%d>: %s", statusCode, *resImport))
	}

	if err := awscommon.WaitUntilResouceAvailable(0, 0, 1, func() (bool, error) {
		_, status, err := t.getClusterInfo()
		if err != nil {
			return false, err
		}
		if status == nil {
			return false, errors.New(fmt.Sprintf("No cluster found[%s]", t.clusterName))
		}
		if *status == "AVAILABLE" {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}

func (t *TiDBCloudAPI) GetImportTaskRoleInfo() (*string, *string, error) {
	fmt.Printf("Project ID: %s, Cluster ID: %s \n\n\n", t.projectId, t.clusterId)
	response, err := t.client.GetImportTaskRoleInfoWithResponse(context.Background(), t.projectId, t.clusterId)
	if err != nil {
		return nil, nil, err
	}
	// fmt.Printf("The response is : %s and %s  \n\n\n", response.JSON200.AwsImportRole.AccountId, response.JSON200.AwsImportRole.ExternalId)

	// for _, item := range response.JSON200.Items {
	// 	fmt.Printf("The item: %#v \n\n\n", item)
	// 	// if c.clusterName == *item.Name {
	// 	// 	return &item.Id, nil
	// 	// }
	// }
	return &response.JSON200.AwsImportRole.AccountId, &response.JSON200.AwsImportRole.ExternalId, nil
}
