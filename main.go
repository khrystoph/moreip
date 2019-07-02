//moreip returns your ipv4 or ipv6 address.
package main

import (
	"io"
	"log"
	"net/http"
	"os"
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
)

const (
	cert = "cert.pem"
	key  = "key.pem"
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

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, req.RemoteAddr)
	})

	err := http.ListenAndServeTLS(":443", cert, key, nil)
	Error.Fatal(err)
	return
}
