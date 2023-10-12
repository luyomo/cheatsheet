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

package kms

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"

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

type RdsAPI struct {
	client *rds.Client

	mapArgs *map[string]string
}

func NewRdsAPI(mapArgs *map[string]string) (*RdsAPI, error) {
	rdsapi := RdsAPI{}

	if mapArgs != nil {
		rdsapi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	rdsapi.client = rds.NewFromConfig(cfg)

	return &rdsapi, nil
}

func (r *RdsAPI) ExportSnapshot2S3(taskName string, snapshotArn, kmsId, roleArn *string, s3Url string) (*types.ExportTask, error) {
	// Check the snapshot exist
	exportTask, err := r.GetLatestExportTaskBySnapshot(snapshotArn)
	if err != nil {
		return nil, err
	}
	if exportTask != nil {
		return exportTask, nil
	}

	parsedS3Dir, err := url.Parse(s3Url)
	if err != nil {
		return nil, err
	}

	taskIdentifier := fmt.Sprintf("%s-%s", taskName, time.Now().Format("20060102150405"))

	if _, err := r.client.StartExportTask(context.TODO(), &rds.StartExportTaskInput{
		ExportTaskIdentifier: aws.String(taskIdentifier),
		IamRoleArn:           roleArn,
		KmsKeyId:             kmsId,
		S3BucketName:         aws.String(parsedS3Dir.Host),
		S3Prefix:             aws.String(strings.Trim(parsedS3Dir.Path, "/")),
		SourceArn:            snapshotArn,
	}); err != nil {
		return nil, err
	}

	if err := awscommon.WaitUntilResouceAvailable(0, 0, 1, func() (bool, error) {
		exportTask, err := r.GetLatestExportTaskBySnapshot(snapshotArn)
		if err != nil {
			return false, err
		}

		if err != nil {
			return false, err
		}
		if exportTask == nil {
			return false, errors.New(fmt.Sprintf("No task created[%s]", taskName))
		}
		// https://github.com/aws/aws-sdk-go-v2/blob/main/service/rds/types/types.go
		if *exportTask.Status == "COMPLETE" {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return nil, err
	}

	exportTask, err = r.GetLatestExportTaskBySnapshot(snapshotArn)
	if err != nil {
		return nil, err
	}

	return exportTask, nil
}

func (r *RdsAPI) GetLatestSnapshot(clusterName string) (*string, error) {
	rdsSnapshot, err := r.client.DescribeDBClusterSnapshots(context.TODO(), &rds.DescribeDBClusterSnapshotsInput{
		DBClusterIdentifier: aws.String(clusterName),
		SnapshotType:        aws.String("manual"),
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(rdsSnapshot.DBClusterSnapshots, func(i, j int) bool {
		return (*rdsSnapshot.DBClusterSnapshots[i].SnapshotCreateTime).Format("20060102150405") > (*rdsSnapshot.DBClusterSnapshots[j].SnapshotCreateTime).Format("20060102150405")
	})

	if len(rdsSnapshot.DBClusterSnapshots) == 0 {
		return nil, nil
	}
	return rdsSnapshot.DBClusterSnapshots[0].DBClusterSnapshotArn, nil

}

func (r *RdsAPI) GetSnapshotByBinlog(clusterName, binlogFile string, binlogPos int64) (*types.DBClusterSnapshot, error) {
	rdsSnapshot, err := r.client.DescribeDBClusterSnapshots(context.TODO(), &rds.DescribeDBClusterSnapshotsInput{
		DBClusterIdentifier: aws.String(clusterName),
		SnapshotType:        aws.String("manual"),
	})
	if err != nil {
		return nil, err
	}

	for _, snapshot := range rdsSnapshot.DBClusterSnapshots {
		theBinlogFile := ""
		theBinlogPos := int64(0)
		for _, tag := range snapshot.TagList {
			switch *tag.Key {
			case "File":
				theBinlogFile = *tag.Value
			case "Position":
				theBinlogPos, err = strconv.ParseInt(*tag.Value, 10, 64)
				if err != nil {
					return nil, err
				}
			}
		}

		if binlogFile == theBinlogFile && binlogPos == theBinlogPos {
			return &snapshot, nil
		}
	}

	return nil, nil
}

func (r *RdsAPI) GetLatestExportTaskBySnapshot(snapshotArn *string) (*types.ExportTask, error) {
	taskResp, err := r.client.DescribeExportTasks(context.TODO(), &rds.DescribeExportTasksInput{SourceArn: snapshotArn})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(taskResp.ExportTasks, func(i, j int) bool {
		return (*taskResp.ExportTasks[i].TaskStartTime).Format("20060102150405") > (*taskResp.ExportTasks[j].TaskStartTime).Format("20060102150405")
	})

	if len(taskResp.ExportTasks) == 0 {
		return nil, nil
	}

	return &taskResp.ExportTasks[0], nil
}

func (r *RdsAPI) RDSSnapshotTaken(clusterName, binlogFile string, binlogPos int64) (*string, error) {

	snapshot, err := r.GetSnapshotByBinlog(clusterName, binlogFile, binlogPos)
	if err != nil {
		return nil, err
	}
	if snapshot != nil {
		return snapshot.DBClusterSnapshotArn, nil
	}

	var tags []types.Tag
	tags = append(tags, types.Tag{Key: aws.String("File"), Value: aws.String(binlogFile)})
	tags = append(tags, types.Tag{Key: aws.String("Position"), Value: aws.String(fmt.Sprintf("%d", binlogPos))})

	backupName := fmt.Sprintf("%s-%s-%d", clusterName, strings.ReplaceAll(binlogFile, ".", "-"), binlogPos)

	createdSnapshot, err := r.client.CreateDBClusterSnapshot(context.TODO(), &rds.CreateDBClusterSnapshotInput{
		DBClusterIdentifier:         aws.String(clusterName),
		DBClusterSnapshotIdentifier: aws.String(backupName),
		Tags:                        tags,
	})
	if err != nil {
		return nil, err
	}

	if err := awscommon.WaitUntilResouceAvailable(0, 0, 1, func() (bool, error) {
		snapshot, err := r.GetSnapshotByBinlog(clusterName, binlogFile, binlogPos)
		if err != nil {
			return false, err
		}
		if snapshot == nil {
			return false, errors.New(fmt.Sprintf("No snapshot created[%s]", backupName))
		}
		// available/copying/creating
		if *snapshot.Status == "available" {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return nil, err
	}

	return createdSnapshot.DBClusterSnapshot.DBClusterSnapshotArn, nil
}
