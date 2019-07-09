//moreip returns your ipv4 or ipv6 address.
package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/crypto/acme/autocert"
)

var (
	Trace         *log.Logger
	Info          *log.Logger
	Warning       *log.Logger
	Error         *log.Logger
	traceHandle   io.Writer
	infoHandle    io.Writer = os.Stdout
	warningHandle io.Writer = os.Stderr
	errorHandle   io.Writer = os.Stderr
	domain        string
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

}

func main() {
	flag.Parse()

	if domain == "example.com" {
		Error.Fatal("Please set the domain via domain flag.")
	}

	ipv4 := strings.Join([]string{"ipv4", domain}, ".")
	ipv6 := strings.Join([]string{"ipv6", domain}, ".")

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

	go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

	log.Fatal(moreIPServer.ListenAndServeTLS("", ""))
	return
}
