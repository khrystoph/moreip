//s3PStore is helper functions to pull certs from s3
package s3PStore

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func ListOjbects() {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewSharedCredentials("", sessionProfile),
	})
	if err != nil {
		fmt.Println("Error setting up session.", err)
		os.Exit(1)
	}
	svc := s3.New(sess)
	input := &s3.ListObjectsInput{
		Bucket:  &s3bucket,
		MaxKeys: aws.Int64(2),
		Prefix:  &filePrefix,
	}

	result, err := svc.ListObjects(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				fmt.Println(s3.ErrCodeNoSuchBucket, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(awserr.Error(aerr))
		}
		return
	}

	fmt.Println(*result.Contents[0].Key)
}
