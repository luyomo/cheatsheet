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

package iam

import (
	"context"
	"errors"
	"fmt"
	// "log"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
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

type IAMAPI struct {
	client *iam.Client

	mapArgs *map[string]string
}

func NewIAMAPI(mapArgs *map[string]string) (*IAMAPI, error) {
	iamapi := IAMAPI{}

	if mapArgs != nil {
		iamapi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	iamapi.client = iam.NewFromConfig(cfg)

	return &iamapi, nil
}

// Return: (RolwArn, error)
func (b *IAMAPI) CreateRole4S3ByRDS(roleName string, path *string, s3backupDir string, kmsArn *string, tags *[]types.Tag) (*string, error) {
	parsedS3Dir, err := url.Parse(s3backupDir)
	if err != nil {
		return nil, err
	}

	policy := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "ExportPolicy",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject*",
                "s3:ListBucket",
                "s3:GetObject*",
                "s3:DeleteObject*",
                "s3:GetBucketLocation"
            ],
            "Resource": [
                "arn:aws:s3:::%s",
                "arn:aws:s3:::%s/*"
            ]
        },
        {
            "Sid": "AllowKMSkey",
            "Effect": "Allow",
            "Action": [
                "kms:Decrypt",
                "kms:Encrypt",
                "kms:GenerateDataKey",
                "kms:GenerateDataKeyWithoutPlaintext",
                "kms:ReEncryptFrom",
                "kms:ReEncryptTo",
                "kms:CreateGrant",
                "kms:DescribeKey",
                "kms:RetireGrant"
            ],
            "Resource": "%s"
        }
    ]
}`, parsedS3Dir.Host, strings.Trim(parsedS3Dir.Host, "/"), *kmsArn)

	policyArn, err := b.createPolicy(roleName, policy, path, nil)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("The policy arn is: <%#v> \n\n\n", *policyArn)

	assumeRolePolicyDocument := `{
	  "Version": "2012-10-17",
	  "Statement": [
	    {
	      "Effect": "Allow",
	      "Principal": {
	         "Service": "export.rds.amazonaws.com"
	       },
	      "Action": "sts:AssumeRole"
	    }
	  ]
	}`

	roleArn, err := b.createRole(roleName, assumeRolePolicyDocument, path, nil)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("The role ARN is: <%#v>\n\n\n", *roleArn)

	if _, err = b.client.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: policyArn,
		RoleName:  aws.String(roleName),
	}); err != nil {
		return nil, err
	}

	return roleArn, nil
}

func (b *IAMAPI) CreateRole4S3External(roleName string, path, kmsArn, accountId, externalId *string, s3backupDir string, tags *[]types.Tag) (*string, error) {

	parsedS3Url, err := url.Parse(s3backupDir)
	if err != nil {
		return nil, err
	}

	importPolicy := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:GetObjectVersion"
            ],
            "Resource": "arn:aws:s3:::%s/*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetBucketLocation"
            ],
            "Resource": "arn:aws:s3:::%s"
        },
        {
            "Sid": "AllowKMSkey",
            "Effect": "Allow",
            "Action": [
                "kms:Decrypt"
            ],
            "Resource": "%s"
        }
    ]
}`, parsedS3Url.Host, parsedS3Url.Host, *kmsArn)

	policyArn, err := b.createPolicy(roleName, importPolicy, path, nil)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("The policyArn: %s \n\n\n", policyArn)

	importAssumeRolePolicyDocument := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Principal": {
                "AWS": "%s"
            },
            "Condition": {
                "StringEquals": {
                    "sts:ExternalId": "%s"
                }
            }
        }
    ]
}`, *accountId, *externalId)

	roleArn, err := b.createRole(roleName, importAssumeRolePolicyDocument, path, nil)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("The role ARN is: <%#v>\n\n\n", *roleArn)

	if _, err = b.client.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: policyArn,
		RoleName:  aws.String(roleName),
	}); err != nil {
		return nil, err
	}
	fmt.Printf("The role is : <%#s> \n\n\n", *roleArn)

	return roleArn, nil
}

func (b *IAMAPI) createPolicy(policyName, policyDocument string, path *string, tags *[]types.Tag) (*string, error) {
	policyArn, err := b.getPolicy(policyName, path)
	if err != nil {
		return nil, err
	}
	if policyArn != nil {
		return policyArn, nil
	}

	createPolicyInput := &iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	}

	if path != nil {
		createPolicyInput.Path = aws.String(fmt.Sprintf("/%s/", *path))
	}
	if tags != nil {
		createPolicyInput.Tags = *tags
	}

	policyEntity, err := b.client.CreatePolicy(context.TODO(), createPolicyInput)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("The policy entity: <%#v> \n\n\n", policyEntity.Policy.Arn)
	return policyEntity.Policy.Arn, nil
}

func (b *IAMAPI) deletePolicy(policyName string, path *string) error {
	policyArn, err := b.getPolicy(policyName, path)
	if err != nil {
		return err
	}

	_, err = b.client.DeletePolicy(context.TODO(), &iam.DeletePolicyInput{PolicyArn: policyArn})
	if err != nil {
		return err
	}

	return nil
}

// Return: (policyArn, error)
func (b *IAMAPI) getPolicy(policyName string, path *string) (*string, error) {
	listPoliciesInput := &iam.ListPoliciesInput{}
	if path != nil {
		listPoliciesInput.PathPrefix = aws.String(fmt.Sprintf("/%s/", *path))
	}

	resp, err := b.client.ListPolicies(context.TODO(), listPoliciesInput)
	if err != nil {
		return nil, err
	}

	for _, policy := range resp.Policies {
		if *policy.PolicyName == policyName {
			return policy.Arn, nil
		}
	}
	return nil, nil
}

func (b *IAMAPI) createRole(roleName, assumeRolePolicyDocument string, path *string, tags *[]types.Tag) (*string, error) {
	roleArn, err := b.getRole(roleName, path)
	if err != nil {
		return nil, err
	}
	if roleArn != nil {
		return roleArn, nil
	}

	createRoleInput := &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
	}

	if path != nil {
		createRoleInput.Path = aws.String(fmt.Sprintf("/%s/", *path))
	}
	if tags != nil {
		createRoleInput.Tags = *tags
	}

	roleEntity, err := b.client.CreateRole(context.TODO(), createRoleInput)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("The policy entity: <%#v> \n\n\n", roleEntity.Role.Arn)
	return roleEntity.Role.Arn, nil
}

func (b *IAMAPI) detachAndRemoveAllRolePolicies(roleName string) error {
	policies, err := b.client.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{RoleName: aws.String(roleName)})
	if err != nil {
		return err
	}
	for _, policy := range policies.AttachedPolicies {
		_, err := b.client.DetachRolePolicy(context.TODO(), &iam.DetachRolePolicyInput{RoleName: aws.String(roleName), PolicyArn: policy.PolicyArn})
		if err != nil {
			return err
		}

		_, err = b.client.DeletePolicy(context.TODO(), &iam.DeletePolicyInput{PolicyArn: policy.PolicyArn})
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *IAMAPI) DeleteRole(roleName string, pathPrefix *string) error {
	roleArn, err := b.getRole(roleName, pathPrefix)
	if err != nil {
		return err
	}
	if roleArn == nil {
		return nil
	}

	if err := b.detachAndRemoveAllRolePolicies(roleName); err != nil {
		return err
	}

	_, err = b.client.DeleteRole(context.TODO(), &iam.DeleteRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		return err
	}
	return nil
}

// Return : (roleArn, error)
func (b *IAMAPI) getRole(roleName string, pathPrefix *string) (*string, error) {
	listRolesInput := &iam.ListRolesInput{}
	if pathPrefix != nil {
		listRolesInput.PathPrefix = aws.String(fmt.Sprintf("/%s/", *pathPrefix))
	}
	resp, err := b.client.ListRoles(context.TODO(), listRolesInput)
	if err != nil {
		return nil, err
	}

	for _, role := range resp.Roles {
		if *role.RoleName == roleName {
			return role.Arn, nil
		}
	}
	return nil, nil
}

func (b *IAMAPI) GetRole(pathPrefix, roleName string) (*[]types.Role, error) {
	resp, err := b.client.ListRoles(context.TODO(), &iam.ListRolesInput{
		PathPrefix: aws.String(fmt.Sprintf("/%s/", pathPrefix)),
	})
	if err != nil {
		return nil, err
	}

	resRoles := []types.Role{}
	for _, role := range resp.Roles {
		if *role.RoleName == roleName {
			resRoles = append(resRoles, role)
		}
	}
	switch len(resRoles) {
	case 0:
		return nil, nil
	case 1:
		return &resRoles, nil
	default:
		return nil, errors.New("Multiple roles matched.")
	}

}

func (c *IAMAPI) makeTags() *[]types.Tag {
	var tags []types.Tag
	if c.mapArgs == nil {
		return &tags
	}

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			tags = append(tags, types.Tag{Key: aws.String(tagName), Value: aws.String(tagValue)})
		}
	}

	return &tags
}

func MakeRoleName(clusterName, subClusterType string) string {
	return fmt.Sprintf("%s.%s", clusterName, subClusterType)
}
