//moreip returns your ipv4 or ipv6 address.
package main

import (
	"crypto/tls"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"s3pstore"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/crypto/acme/autocert"
)

var (
	//Trace is log handling for Trace level messages
	Trace *log.Logger
	//Info is log handling for Info level messaging
	Info *log.Logger
	//Warning is log handling for Warning level messaging
	Warning *log.Logger
	//Error is log handling for Error level messaging
	Error                                                   *log.Logger
	traceHandle                                             io.Writer
	infoHandle                                              io.Writer = os.Stdout
	warningHandle                                           io.Writer = os.Stderr
	errorHandle                                             io.Writer = os.Stderr
	s3bucket, filePrefix, domain, sessionProfile, awsRegion string
	sess                                                    *session.Session
	s3Sess                                                  *s3.S3
)

const (
	certDir     = "certs"
	moreIPImage = "moreip.jpg"
	//ProviderName is an exported const to identify when the EC2RoleProvider is being used
	ProviderName = "EC2RoleProvider"
	sleepConst   = 30
)

type localFile struct {
	filename    string
	fileModTime time.Time
}

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

	flag.StringVar(&domain, "d", "example.com", "enter your fully qualified domain name here. Default: example.com")
	flag.StringVar(&domain, "domain", "example.com", "enter your fully qualified domain name here. Default: example.com")
	flag.StringVar(&awsRegion, "region", "", "Enter region you wish to connect with. Default: \"\"")
	flag.StringVar(&awsRegion, "r", "", "Enter region you wish to connect with. Default: \"\"")
	flag.StringVar(&s3bucket, "bucket", "bucket.example.com", "Enter your s3 bucket to pull from here.")
	flag.StringVar(&s3bucket, "b", "bucket.example.com", "Enter your s3 bucket to pull from here.")
	flag.StringVar(&filePrefix, "prefix", "certs", "Enter the object prefix where you stored the certs.")
	flag.StringVar(&sessionProfile, "profile", "default", "enter the profile you wish to use to connect. Default: default")
	flag.StringVar(&sessionProfile, "p", "default", "enter the profile you wish to use to connect. Default: default")
}

func main() {
	flag.Parse()

	s3pstore.FilePrefix = filePrefix
	s3pstore.S3bucket = s3bucket
	s3pstore.SessionProfile = sessionProfile

	if domain == "example.com" {
		if domainenv := os.Getenv("DOMAIN"); domainenv != "" {
			domain = domainenv
		} else {
			Error.Printf("Domain value: %s\n$DOMAIN = %s", domain, domainenv)
			Error.Fatal("Please set the domain via domain flag or set DOMAIN env var.")
		}
	}

	if s3bucket == "bucket.example.com" && os.Getenv("OBJECTBUCKET") == "" {
		Error.Printf("Bucket value: %s\n$OBJECTBUCKET = %s", s3bucket, os.Getenv("OBJECTBUCKET"))
		Error.Fatalf("Object bucket not set. Please set OBJECTBUCKET env var or via -b flag")
	} else if os.Getenv("OBJECTBUCKET") != "" && s3bucket == "bucket.example.com" {
		s3bucket = os.Getenv("OBJECTBUCKET")
	}

	if awsRegion == "" && os.Getenv("REGION") == "" {
		Error.Fatalf("region var and $REGION env var are both empty. Please set the flag OR environment variable.")
	} else if os.Getenv("REGION") != "" && awsRegion == "" {
		awsRegion = os.Getenv("REGION")
	}

	ipv4 := strings.Join([]string{"ipv4", domain}, ".")
	ipv6 := strings.Join([]string{"ipv6", domain}, ".")

	Info.Println("ipv4: " + ipv4)
	Info.Println("ipv6: " + ipv6)

	if _, err := os.Stat("certs"); os.IsNotExist(err) {
		Info.Println("certs dir does not exist. Creating it and calling pullCache.")
		os.Mkdir("certs", 0755)
		Info.Println("pulling certs if they exist in cache.")
	}

	//make map to store local cached files in to check against returned objects.
	//if object in remote cache does not exist in map, pull the cert to the local filesystem.
	fileMap := make(map[string]os.FileInfo)
	cachedObjectsMap := make(map[string]s3.Object)

	//List S3 Objects for use to check whether files should be pulled from cache or not
	cachedObjects, err := s3pstore.ListObjects(s3bucket, filePrefix)
	if err != nil {
		Info.Printf("Error calling list objects: %v\n", err)
	}

	//if we have file in cache, but not local to filesystem, download it
	if fileList, err := ioutil.ReadDir("certs"); err != nil {
		Info.Printf("Had issues reading certs directory\n")
	} else if len(fileList) > 0 {
		for _, files := range fileList {
			fileMap[files.Name()] = files
		}
		//we want to check if the object that we saw in the cache exists on the filesystem, if not, then we
		//pull the cert down from the cache into the filesystem. We also want to map the cache objects
		for _, cacheItem := range cachedObjects.Contents {
			if _, ok := fileMap[strings.Split(*cacheItem.Key, "certs/")[1]]; ok != true {
				s3pstore.PullObjects(strings.Split(*cacheItem.Key, "certs/")[1], "certs/")
			}
			cachedObjectsMap[*cacheItem.Key] = *cacheItem
		}
		Info.Printf("File Map Len:%v\nCached Objects Len:%v\n", len(fileMap), len(cachedObjectsMap))
		if len(fileMap) > len(cachedObjectsMap) {
			for _, localToCache := range fileMap {
				if _, ok := cachedObjectsMap[localToCache.Name()]; !ok {
					s3pstore.PushCerts(localToCache.Name(), s3bucket)
				}
			}
		}
	} else if len(fileList) == 0 {
		for _, file := range cachedObjects.Contents {
			s3pstore.PullObjects(strings.Split(*file.Key, "certs/")[1], "certs/")
		}
	}

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain, ipv4, ipv6),
		Cache:      autocert.DirCache(certDir),
	}

	//TODO: add handler function for jpeg

	//http.HandleFunc("/moreip", func(w http.ResponseWriter, req *http.Request) {
	//	img, err := os.Open(moreIPImage)
	//})

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if string(req.RemoteAddr[0]) == "[" {
			io.WriteString(w, strings.Trim(strings.Split(req.RemoteAddr, "]")[0], "[]")+"\n")
		} else {
			io.WriteString(w, strings.Split(req.RemoteAddr, ":")[0]+"\n")
		}
	})

	moreIPServer := &http.Server{
		Addr: ":https",
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	var wg sync.WaitGroup

	Info.Printf("Starting the letsencrypt server\n")
	go func() {
		wg.Add(1)
		defer wg.Done()
		http.ListenAndServe(":http", certManager.HTTPHandler(nil))
	}()

	Info.Printf("Entering into cache handling infinite loop.\n")
	loopCounter := 0
	go func() {
		wg.Add(1)
		defer wg.Done()
		localFiles := make(map[string]localFile)
		for true {
			loopCounter++
			Info.Printf("cacheHandling loop at end of program #%v.\n", loopCounter)
			fileList, err := ioutil.ReadDir("certs")
			if err != nil {
				Info.Printf("%v\n", err)
			}
			if len(fileList) > 0 {
				for _, files := range fileList {
					if _, ok := localFiles[files.Name()]; ok != true {
						localFiles[files.Name()] = localFile{
							filename:    files.Name(),
							fileModTime: files.ModTime(),
						}
						s3pstore.PushCerts(files.Name(), s3bucket)
					} else if ok == true && localFiles[files.Name()].fileModTime.Before(files.ModTime()) {
						localFiles[files.Name()] = localFile{
							filename:    files.Name(),
							fileModTime: files.ModTime(),
						}
						s3pstore.PushCerts(files.Name(), s3bucket)
					}
				}
			}
			time.Sleep(sleepConst * time.Second)
		}
	}()
	Info.Printf("Starting the main TLS server.\n")
	Error.Fatal(moreIPServer.ListenAndServeTLS("", ""))

	return
}
