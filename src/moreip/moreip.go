//moreip returns your ipv4 or ipv6 address.
package main

import (
	"crypto/tls"
	"flag"
	"io"
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
	sleepConst   = 5
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

	flag.StringVar(&domain, "d", "example.com", "enter your fully qualified domain name here. Default: example.com")
	flag.StringVar(&domain, "domain", "example.com", "enter your fully qualified domain name here. Default: example.com")
	flag.StringVar(&awsRegion, "region", "us-east-1", "Enter region you wish to connect with. Default: us-east-1")
	flag.StringVar(&awsRegion, "r", "us-east-1", "Enter region you wish to connect with. Default: us-east-1")
	flag.StringVar(&s3bucket, "bucket", "moreip.jbecomputersolutions.com", "Enter your s3 bucket to pull from here.")
	flag.StringVar(&s3bucket, "b", "moreip.jbecomputersolutions.com", "Enter your s3 bucket to pull from here.")
	flag.StringVar(&filePrefix, "prefix", "certs", "Enter the object prefix where you stored the certs.")
	flag.StringVar(&sessionProfile, "profile", "default", "enter the profile you wish to use to connect. Default: default")
	flag.StringVar(&sessionProfile, "p", "default", "enter the profile you wish to use to connect. Default: default")
}

func main() {
	flag.Parse()

	s3pstore.FilePrefix = filePrefix
	s3pstore.S3bucket = s3bucket

	if domain == "example.com" {
		Error.Fatal("Please set the domain via domain flag.")
	}

	ipv4 := strings.Join([]string{"ipv4", domain}, ".")
	ipv6 := strings.Join([]string{"ipv6", domain}, ".")

	Info.Println("ipv4: " + ipv4)
	Info.Println("ipv6: " + ipv6)

	if _, err := os.Stat("certs/" + ipv4); os.IsNotExist(err) {
		Info.Println("certs do not exist. Creating them.")
		if _, err := os.Stat("certs"); os.IsNotExist(err) {
			Info.Println("certs dir does not exist. creating")
			os.Mkdir("certs", 0755)
		}
		Info.Println("entering cache handler.")
		err := s3pstore.CacheHandler(ipv4)
		if err != nil {
			Error.Println(err)
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
	go func() {
		wg.Add(1)
		defer wg.Done()
		http.ListenAndServe(":http", certManager.HTTPHandler(nil))
	}()

	Info.Printf("Starting the main TLS server.\n")
	go func() {
		log.Fatal(moreIPServer.ListenAndServeTLS("", ""))
	}()
	Info.Printf("Entering into cache handling infinite loop.\n")
	loopCounter := 0
	for true {
		loopCounter++
		Info.Printf("cacheHandling loop at end of program #%v.\n", loopCounter)
		if _, err := os.Stat("certs/" + ipv4); os.IsNotExist(err) {
			err := s3pstore.CacheHandler(ipv4)
			if err != nil {
				Error.Println(err)
			}
		}
		if _, err := os.Stat("certs/" + ipv6); os.IsNotExist(err) {
			err := s3pstore.CacheHandler(ipv6)
			if err != nil {
				Error.Println(err)
			}
		}
		Info.Printf("certs/" + domain)
		if _, err := os.Stat("certs/" + domain); os.IsNotExist(err) {
			err := s3pstore.CacheHandler(domain)
			if err != nil {
				Error.Println(err)
			}
		}
		time.Sleep(sleepConst * time.Second)
	}
	wg.Wait()

	return
}
