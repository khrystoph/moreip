//Package s3pstore is a series of helper functions to pull certs from s3 and push updated certs to s3
package s3pstore

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	certDir     = "certs"
	awsRegion   = "us-west-2"
	maxKeyCount = 3
)

var (
	//S3bucket is an exported variable because we need to set it in main
	S3bucket string
	//FilePrefix is Exported to be set by main
	FilePrefix     string
	SessionProfile string
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
	sessionMust             = session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ec2RoleProvider = &ec2rolecreds.EC2RoleProvider{
		Client: ec2metadata.New(sessionMust, &aws.Config{
			HTTPClient: &http.Client{Timeout: 10 * time.Second},
		}),
		ExpiryWindow: 0,
	}
	sharedCreds = &credentials.SharedCredentialsProvider{
		Profile: SessionProfile,
	}
	creds     = credentials.NewChainCredentials([]credentials.Provider{ec2RoleProvider, sharedCreds})
	awsConfig = &aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: creds,
	}
)

func init() {
	Trace = log.New(traceHandle,
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

type certStruct struct {
	name    string
	modTime time.Time
}

func awsSessionHandler(config *aws.Config, creds *credentials.Credentials) (err error) {
	sess, err = session.NewSession(awsConfig)
	if err != nil {
		Error.Println("Error creating session.")
		return err
	}

	_, err = sess.Config.Credentials.Get()
	if err != nil {
		Error.Println("error retrieving credentials. Profile name: ", SessionProfile)
		Error.Println("Error msg: ", err)
		return err
	}

	return nil
}

//IsEmpty checks if the directory is empty to determine if we should pull certs or push certs
func IsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func listObjects() (objectList *s3.ListObjectsV2Output, err error) {
	err = awsSessionHandler(awsConfig, creds)
	if err != nil {
		fmt.Println("Error setting up session.", err)
		return nil, err
	}
	svc := s3.New(sess)
	Info.Printf("s3 bucket:%v\n", S3bucket)
	Info.Printf("FilePrefix: %v\n", FilePrefix)
	prefix := FilePrefix + "/"
	input := &s3.ListObjectsV2Input{
		Bucket: &S3bucket,
		//MaxKeys: aws.Int64(maxKeyCount),
		Prefix: &prefix,
	}

	Info.Printf("listObjectsInput:\n%v\n", input)

	result, err := svc.ListObjectsV2(input)
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
	Info.Printf("Exiting listObjects\n")
	return result, nil
}

//ListObjects is an exported call to list objects in a bucket
func ListObjects(S3bucket string, FilePrefix string) (objectList *s3.ListObjectsV2Output, err error) {
	err = awsSessionHandler(awsConfig, creds)
	if err != nil {
		fmt.Println("Error setting up session.", err)
		return nil, err
	}
	svc := s3.New(sess)
	Info.Printf("s3 bucket:%v\n", S3bucket)
	Info.Printf("FilePrefix: %v\n", FilePrefix)
	input := &s3.ListObjectsV2Input{
		Bucket:  &S3bucket,
		MaxKeys: aws.Int64(maxKeyCount),
		Prefix:  &FilePrefix,
	}

	Info.Printf("listObjectsInput:\n%v\n", input)

	result, err := svc.ListObjectsV2(input)
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
	Info.Printf("Exiting listObjects\n")
	return result, nil
}

//PullObjects pulls object from s3bucket and stores locally in cache
func PullObjects(cacheFile string, prefix string) (err error) {
	err = awsSessionHandler(awsConfig, creds)
	if err != nil {
		return err
	}
	if _, err = os.Stat(certDir); os.IsNotExist(err) {
		err = os.Mkdir(certDir, 0755)
	}
	if err != nil {
		return err
	}
	inputKey := prefix + cacheFile
	downloader := s3manager.NewDownloader(sess)
	Info.Printf("%v\n", cacheFile)
	input := &s3.GetObjectInput{
		Bucket:  &S3bucket,
		Key:     &inputKey,
		IfMatch: &inputKey,
	}

	Info.Printf("certfile: %v\n", inputKey)
	f, err := os.Create(inputKey)
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

	Info.Printf("Exiting pullObjects.\n")
	return nil
}

//syncObjects pulls (or pushes) objects to or from s3 bucket/prefix.
/*func pullObjects(certs *s3.ListObjectsV2Output) (err error) {
	err = awsSessionHandler(awsConfig, creds)
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
	if len(certs.Contents) == 1 && *certs.Contents[0].Key == "certs/" {
		return errors.New("cert contents list only contains directory")
	}
	Info.Printf("%v\n", certs)
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
	Info.Printf("Exiting pullObjects.\n")
	return nil
}*/

//PushCerts allows external libraries to push a cert to S3
func PushCerts(cert string, bucket string) (err error) {
	err = awsSessionHandler(awsConfig, creds)

	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)
	f, err := os.Open(FilePrefix + "/" + cert)
	if err != nil {
		return err
	}

	Info.Println(FilePrefix)

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
	Info.Printf("Exiting pushCerts.")
	return nil
}

/*
func pushCerts(cert string, bucket string) (err error) {
	err = awsSessionHandler(awsConfig, creds)

	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)
	f, err := os.Open(FilePrefix + "/" + cert)
	if err != nil {
		return err
	}

	Info.Println(FilePrefix)

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
	Info.Printf("Exiting pushCerts.")
	return nil
}*/

//CacheHandler is a function that handles pushing and pulling certs from s3 to handle SSL/TLS for the http server
/*func CacheHandler(cert string) (err error) {
	var fileModTime []certStruct

	Info.Println("cache handler test")

	certList, err := listObjects()
	if err != nil {
		Error.Printf("%v\n", certList)
		Error.Printf("error calling listObjects\n")
		Error.Printf("%v\n", err)
	}
	Info.Printf("Checking list of certs.\n")
	Info.Printf("ListObjectsOutput:\n%v", certList.Contents)

	//make list of objects in bucket with prefix of "certs/"
	certListMap := make(map[string]s3.Object)
	if certList != nil {
		Info.Printf("Entering into certMap loop.\n")
		for index, object := range certList.Contents {
			certListMap[*certList.Contents[index].Key] = *object
		}
		if _, ok := certListMap[cert]; ok {
			pushCerts(cert, S3bucket)
			return
		}
	}

	Info.Printf("certList:\n%v\n", certList)
	if certList != nil {
		err = pullObjects(certList)
		if err != nil {
			Error.Println("error calling pull objects")
			Error.Printf("%v\n", certList)
			fmt.Println(err)
			return err
		}
	}

	Info.Println("Made it to the cacheFileList and certListMappings.")
	cacheFileList, err := ioutil.ReadDir(certDir)
	Info.Printf("CacheFileList:\n%v\n", cacheFileList)
	if err != nil {
		Error.Println("encountered error listing certs dir.")
		return err
	}

	for _, certFile := range cacheFileList {
		info, name := certFile.ModTime(), certFile.Name()
		fileModTime = append(fileModTime, certStruct{name: name, modTime: info})
	}

	if certList == nil && len(fileModTime) != 0 {
		Info.Printf("Cert List: %v\n", certList)
		for _, certStruct := range fileModTime {
			pushCerts(certStruct.name, S3bucket)
		}
	}

	Info.Printf("Entering into push certs loop.\n")
	for index, certFile := range fileModTime {
		certStat, err := os.Stat(FilePrefix + "/" + certFile.name)
		info, name := certStat.ModTime(), certStat.Name()

		if certname, ok := certListMap[name]; !ok {
			pushCerts(*certname.Key, S3bucket)
		}

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
	Info.Printf("Exiting CacheHandler\n")
	return nil
}*/
