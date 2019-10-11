//Package s3pstore is a series of helper functions to pull certs from s3 and push updated certs to s3
package s3pstore

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	certDir   = "certs"
	awsRegion = "us-west-2"
)

var (
	//S3bucket is an exported variable because we need to set it in main
	S3bucket string
	//FilePrefix is Exported to be set by main
	FilePrefix     string
	sessionProfile string
	sess           *session.Session
	//Trace is a setting for logging to allow for a specific log type of trace level logging
	Trace *log.Logger
	//Info is a setting for logging to allow for a specific log type of info level logging
	Info *log.Logger
	//Warning is a setting for logging to allow for a specific log type of warning level logging
	Warning *log.Logger
	//Error is a setting for logging to allow for a specific log type of error level logging
	Error         *log.Logger
	traceHandle   io.Writer
	infoHandle    io.Writer = os.Stdout
	warningHandle io.Writer = os.Stderr
	errorHandle   io.Writer = os.Stderr
)

type certStruct struct {
	name    string
	modTime time.Time
}

func listOjbects() (objectList *s3.ListObjectsOutput, err error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewSharedCredentials("", sessionProfile),
	})
	if err != nil {
		fmt.Println("Error setting up session.", err)
		os.Exit(1)
	}
	svc := s3.New(sess)
	input := &s3.ListObjectsInput{
		Bucket:  &S3bucket,
		MaxKeys: aws.Int64(2),
		Prefix:  &FilePrefix,
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

	if len(result.Contents) == 0 {
		return nil, errors.New("no ojbects found in bucket/prefix")
	}
	return result, nil
}

//syncObjects pulls (or pushes) objects to or from s3 bucket/prefix.
func pullObjects(certs *s3.ListObjectsOutput) (err error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewSharedCredentials("", sessionProfile),
	})
	if err != nil {
		return err
	}
	if _, err = os.Stat(certDir); os.IsNotExist(err) {
		err = os.Mkdir(certDir, 0755)
	}
	if err != nil {
		return err
	}
	downloader := s3manager.NewDownloader(sess)
	for object := range certs.Contents {
		input := &s3.GetObjectInput{
			Bucket: &S3bucket,
			Key:    certs.Contents[object].Key,
		}
		certfile := strings.Join([]string{*certs.Contents[object].Key}, "")
		f, err := os.Create(certfile)
		if err != nil {
			return err
		}
		cert, err := downloader.Download(f, input)
		if err != nil {
			return err
		}
		fmt.Printf("Downloaded file, %d bytes\n", cert)
		f.Close()
		f.Sync()

	}

	return nil
}

func pushCerts(cert string, bucket string) (err error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewSharedCredentials("", sessionProfile),
	})

	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)
	f, err := os.Open(FilePrefix + "/" + cert)
	if err != nil {
		return err
	}

	s3objectKey := FilePrefix + "/" + cert
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    &s3objectKey,
		Body:   f,
	})
	if err != nil {
		return err
	}

	fmt.Printf("file uploaded to, %s\n", aws.StringValue(&result.Location))
	return nil
}

func awsSessionHandler(config *aws.Config) (err error) {
	sess, err := session.NewSession(config)
	if err != nil {
		Error.Println("Error creating session.")
		return err
	}

	_, err = sess.Config.Credentials.Get()
	if err != nil {
		Error.Println("error retrieving credentials. Profile name: ", sessionProfile)
		Error.Println("Error msg: ", err)
		return err
	}

	return nil
}

//CacheHandler is a function that handles pushing and pulling certs from s3 to handle SSL/TLS for the http server
func CacheHandler(sess *session.Session, cert string) (err error) {
	var fileModTime []certStruct
	flag.Parse()

	certList, err := listOjbects()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = pullObjects(certList)
	if err != nil {
		fmt.Println(err)
		return err
	}
	cacheFileList, err := ioutil.ReadDir(certDir)

	for _, certFile := range cacheFileList {
		info, name := certFile.ModTime(), certFile.Name()
		fileModTime = append(fileModTime, certStruct{name: name, modTime: info})
	}
	//fmt.Printf("%v", fileStatModTime)

	for index, certFile := range fileModTime {
		certStat, err := os.Stat(FilePrefix + "/" + certFile.name)
		info, name := certStat.ModTime(), certStat.Name()
		if fileModTime[index].name != name {
			fmt.Println("file names do not match. Panic.")
			//os.Exit(1)
			err = errors.New("file names do not match when running fileModTime check")
			return err
		}
		fmt.Printf("modTime from original cache file: \t%v\nmodTime after modification: \t\t%v\n", fileModTime[index].modTime, info)
		if info != fileModTime[index].modTime {
			fmt.Printf("file: %s, modified @:\n%v.\n...Updating cache.\n", name, info)
			err = pushCerts(name, S3bucket) //just testing the functionality works for now
			if err != nil {
				fmt.Println(err)
				//os.Exit(1)
				return err
			}
		}
	}
	return nil
}
