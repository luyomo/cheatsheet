package app

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/luyomo/cheatsheet/aurora2tidbcloud/internal/app/configs"
	cfapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/cloudformation"
	iamapilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/iam"
	s3apilib "github.com/luyomo/cheatsheet/aurora2tidbcloud/pkg/aws/s3"
	"log"
)

func Clean(gOpt configs.Options) error {
	log.Printf("Starting to clean all the aws resources \n")

	// 01. Clean role of import
	iamapi, err := iamapilib.NewIAMAPI(nil)
	if err != nil {
		return err
	}

	if err := iamapi.DeleteRole(IMPORT_ROLE, aws.String(MODULE_NAME)); err != nil {
		return err
	}

	// 02. Clean role of export
	if err := iamapi.DeleteRole(EXPORT_ROLE, aws.String(MODULE_NAME)); err != nil {
		return err
	}

	// 04. Empty the bucket

	cfapi, err := cfapilib.NewCFAPI(nil)
	if err != nil {
		return err
	}
	s3arn, err := cfapi.GetStackResource(STACKNAME_DUMPLING, "S3Bucket")
	if err != nil {
		return err
	}
	if s3arn != nil {
		fmt.Printf("The s3 bucket are : <%#v> \n\n\n", *s3arn)
	}

	if s3arn != nil {
		s3api, err := s3apilib.NewS3API(nil)
		if err != nil {
			return err
		}

		if err := s3api.DeleteObject(*s3arn, ""); err != nil {
			return err
		}
	}
	// 05. Remove cloudformation for binlog
	if err := cfapi.DestroyStack(STACKNAME_DUMPLING); err != nil {
		return err
	}
	// 06. Remove cloudformation for dumpling
	if err := cfapi.DestroyStack(STACKNAME_BINLOG); err != nil {
		return err
	}

	return nil
}
