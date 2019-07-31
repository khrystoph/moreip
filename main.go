//Package moreip returns your ipv4 or ipv6 address.
package moreip

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"s3PStore"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"golang.org/x/crypto/acme/autocert"
)

var (
	Trace                             *log.Logger
	Info                              *log.Logger
	Warning                           *log.Logger
	Error                             *log.Logger
	traceHandle                       io.Writer
	infoHandle                        io.Writer = os.Stdout
	warningHandle                     io.Writer = os.Stderr
	errorHandle                       io.Writer = os.Stderr
	domain, sessionProfile, awsRegion string
	sess                              *session.Session
)

const (
	certDir     = "certs"
	moreIPImage = "moreip.jpg"
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
	flag.StringVar(&sessionProfile, "profile", "default", "enter the profile you wish to use to connect. Default: default")
	flag.StringVar(&sessionProfile, "p", "default", "enter the profile you wish to use to connect. Default: default")
	flag.StringVar(&awsRegion, "region", "us-east-1", "Enter region you wish to connect with. Default: us-east-1")

}

func awsSessionHandler(config *aws.Config) (err error) {
	sess, err = session.NewSession(config)
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

func cacheHandler(session *session) (err error){
	if certList, err := s3PStore.ListObjects(awsRegion, sessionProfile); err != nil {
		return err
	}
	if err := s3PStore.PullObjects(certList)
}

func main() {
	flag.Parse()
	var (
		awsConfig = aws.Config{
			Region:      aws.String(awsRegion),
			Credentials: credentials.NewSharedCredentials("", sessionProfile),
		}
	)

	if domain == "example.com" {
		Error.Fatal("Please set the domain via domain flag.")
	}

	err := awsSessionHandler(&awsConfig)
	if err != nil {
		Error.Fatalln(err)
	}

	ipv4 := strings.Join([]string{"ipv4", domain}, ".")
	ipv6 := strings.Join([]string{"ipv6", domain}, ".")

	if _, err := os.Stat("certs/" + ipv4); os.IsNotExist(err) {
		cacheHandler(sess)
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

	certList := os.Readdir(certDir)
	moreIPServer := &http.Server{
		Addr: ":https",
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

	for certFile := range certList {
		info, err := os.Stat(certFile)
		if info.ModTime != certFile.ModTime {
			err = s3pstore.pushCerts(certFile, s3bucket)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	log.Fatal(moreIPServer.ListenAndServeTLS("", ""))
	return
}
