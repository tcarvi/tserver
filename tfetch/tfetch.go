package tFetch

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
 * 1
 */
func FetchBodyToScreen(url string) {
	// strings.HasPrefix
	if !strings.HasPrefix(url, "http://") {
		url = "http://" + url
	}

	// http.Get
	respStream, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error on http.Get(url):\n %v\n", err)
		os.Exit(1)
	}
	writeDataOfResponseToScreen(respStream)
	writeDataOfResponseToFile(respStream)
	respStream.Body.Close()
}

/*
		FetchManyBodyToScreen method
	    A goroutine is a concurrent function execution.
	    A channel is a communication mechanism that allows
	      one goroutine to pass values of a specified type
		  to another goroutine.
	    The function main() runs in a goroutine and
	      the go statement creates additional goroutines.
*/
func FetchManyBodiesToScreen(urls []string) {
	start := time.Now()
	// Create a channel of string to synchronize goroutines
	ch := make(chan string)
	for _, url := range urls {
		// Starts a new goroutine that calls fetch asynchronously
		go fetch(url, ch)
	}
	for range urls {
		fmt.Println(<-ch) // receive from channel ch
	}
	fmt.Printf("%.2fs elapsed\n", time.Since(start).Seconds())
}

/*
 * 2
 */
func FetchBodyToFile(url string) {
	if !strings.HasPrefix(url, "http://") {
		url = "http://" + url
	}
	respStream, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("respStream.Status = ", respStream.Status)
	fmt.Println("respStream.StatusCode = ", respStream.StatusCode)
	fileName := "tcarvi2.go"
	//fileName := strings.Replace(strings.TrimPrefix(url, "http://"), ".", "_", -1) + ".html"
	file, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	writtenNumber, err := io.Copy(file, respStream.Body)
	respStream.Body.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: reading %s: %v\n", url, err)
		os.Exit(2)
	}
	fmt.Print(writtenNumber)
}

/*
 * Called for goroutine
 */
func fetch(url string, ch chan<- string) {
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		ch <- fmt.Sprint(err) // to to channel ch
		return
	}

	nbytes, err := io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close() //
	if err != nil {
		ch <- fmt.Sprintf("while reading %s: %v", url, err)
		return
	}
	secs := time.Since(start).Seconds()
	ch <- fmt.Sprintf("%.2fs  %7d  %s", secs, nbytes, url)
}

/*
 * Write data of Response to file and screen
 */
func writeDataOfResponseToScreen(resp *http.Response) {

	// Response data
	var buffer bytes.Buffer
	buffer.WriteString("Status = " + resp.Status + "\n\n")
	buffer.WriteString("StatusCode = " + strconv.Itoa(resp.StatusCode) + "\n\n")
	buffer.WriteString("Proto = " + resp.Proto + "\n\n")
	buffer.WriteString("ProtoMajor = " + strconv.Itoa(resp.ProtoMajor) + "\n\n")
	buffer.WriteString("ProtoMinor = " + strconv.Itoa(resp.ProtoMinor) + "\n\n")
	buffer.WriteString("Close = " + strconv.FormatBool(resp.Close) + "\n\n")

	// HTTP Header
	buffer.WriteString("HTTP Header:\n")
	for k, _ := range resp.Header {
		buffer.WriteString(k + " = " + resp.Header.Get(k) + "\n")
	}

	// HTTP ContentLength
	buffer.WriteString("\n")
	buffer.WriteString("ContentLength = " + strconv.FormatInt(resp.ContentLength, 10) + "\n\n")

	// HTTP TransferEncoding
	buffer.WriteString("HTTP TransferEncoding: ")
	buffer.WriteString(strings.Join(resp.TransferEncoding, ", ") + "\n\n")

	// HTTP Trailer
	buffer.WriteString("HTTP Trailer:\n")
	for k, _ := range resp.Trailer {
		buffer.WriteString(k + " = " + resp.Trailer.Get(k) + "\n")
	}

	// HTTP TLS ConnectionState
	buffer.WriteString("\nHTTP TLS ConnectionState:\n")
	if resp.TLS != nil {
		buffer.WriteString("HandshakeComplete:\n")
		buffer.WriteString("DidResume:\n")
		buffer.WriteString("NegotiatedProtocol: " + resp.TLS.NegotiatedProtocol + "\n")
		buffer.WriteString("NegotiatedProtocolIsMutual:\n")
		buffer.WriteString("ServerName: " + resp.TLS.ServerName)
	}

	// HTTP Body
	buffer.WriteString("\nHTTP BODY:\n")
	b, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "Error on ioutil.ReadAll(resp.Body):\n %v\n", err2)
		os.Exit(3)
	}
	_, err3 := buffer.Write(b)
	if err3 != nil {
		fmt.Fprintf(os.Stderr, "Error on buffer.Write(b):\n %v\n", err3)
		os.Exit(4)
	}

	//textBytes := buffer.Bytes()

	//io.Write(textBytes)

}

/*
 * Write data of Response to file and screen
 */
func writeDataOfResponseToFile(resp *http.Response) {
	// Response data
	var buffer bytes.Buffer
	buffer.WriteString("Status = " + resp.Status + "\n\n")
	buffer.WriteString("StatusCode = " + strconv.Itoa(resp.StatusCode) + "\n\n")
	buffer.WriteString("Proto = " + resp.Proto + "\n\n")
	buffer.WriteString("ProtoMajor = " + strconv.Itoa(resp.ProtoMajor) + "\n\n")
	buffer.WriteString("ProtoMinor = " + strconv.Itoa(resp.ProtoMinor) + "\n\n")
	buffer.WriteString("Close = " + strconv.FormatBool(resp.Close) + "\n\n")

	// HTTP Header
	buffer.WriteString("HTTP Header:\n")
	for k, _ := range resp.Header {
		buffer.WriteString(k + " = " + resp.Header.Get(k) + "\n")
	}

	// HTTP ContentLength
	buffer.WriteString("\n")
	buffer.WriteString("ContentLength = " + strconv.FormatInt(resp.ContentLength, 10) + "\n\n")

	// HTTP TransferEncoding
	buffer.WriteString("HTTP TransferEncoding: ")
	buffer.WriteString(strings.Join(resp.TransferEncoding, ", ") + "\n\n")

	// HTTP Trailer
	buffer.WriteString("HTTP Trailer:\n")
	for k, _ := range resp.Trailer {
		buffer.WriteString(k + " = " + resp.Trailer.Get(k) + "\n")
	}

	// HTTP TLS ConnectionState
	buffer.WriteString("\nHTTP TLS ConnectionState:\n")
	if resp.TLS != nil {
		buffer.WriteString("HandshakeComplete:\n")
		buffer.WriteString("DidResume:\n")
		buffer.WriteString("NegotiatedProtocol: " + resp.TLS.NegotiatedProtocol + "\n")
		buffer.WriteString("NegotiatedProtocolIsMutual:\n")
		buffer.WriteString("ServerName: " + resp.TLS.ServerName)
	}

	// HTTP Body
	buffer.WriteString("\nHTTP BODY:\n")
	b, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "Error on ioutil.ReadAll(resp.Body):\n %v\n", err2)
		os.Exit(3)
	}
	_, err3 := buffer.Write(b)
	if err3 != nil {
		fmt.Fprintf(os.Stderr, "Error on buffer.Write(b):\n %v\n", err3)
		os.Exit(4)
	}

	textBytes := buffer.Bytes()
	ioutil.WriteFile("/libs/logs/tFetchHttp.log", textBytes, 0222)
}

/*
 * Usage: tFetchHttp {http://domain}
 */

func mainExecution() {

	// Verify typing of url
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stdout, "\nErro: Falta indicação de url.\n")
		os.Exit(1)
	}

	url := os.Args[1]

	// Verify prefix
	if !strings.HasPrefix(url, "http://") {
		url = "http://" + url
	}

	// GET
	resp, err1 := http.Get(url)
	if err1 != nil {
		fmt.Fprintf(os.Stderr, "Error on http.Get(url):\n %v\n", err1)
		os.Exit(2)
	}

	// Response data
	var buffer bytes.Buffer
	buffer.WriteString("Status = " + resp.Status + "\n\n")
	buffer.WriteString("StatusCode = " + strconv.Itoa(resp.StatusCode) + "\n\n")
	buffer.WriteString("Proto = " + resp.Proto + "\n\n")
	buffer.WriteString("ProtoMajor = " + strconv.Itoa(resp.ProtoMajor) + "\n\n")
	buffer.WriteString("ProtoMinor = " + strconv.Itoa(resp.ProtoMinor) + "\n\n")
	buffer.WriteString("Close = " + strconv.FormatBool(resp.Close) + "\n\n")

	// HTTP Header
	buffer.WriteString("HTTP Header:\n")
	for k, _ := range resp.Header {
		buffer.WriteString(k + " = " + resp.Header.Get(k) + "\n")
	}

	// HTTP ContentLength
	buffer.WriteString("\n")

	buffer.WriteString("ContentLength = " + strconv.FormatInt(resp.ContentLength, 10) + "\n\n")

	// HTTP TransferEncoding
	buffer.WriteString("HTTP TransferEncoding: ")
	buffer.WriteString(strings.Join(resp.TransferEncoding, ", ") + "\n\n")

	// HTTP Trailer
	buffer.WriteString("HTTP Trailer:\n")
	for k, _ := range resp.Trailer {
		buffer.WriteString(k + " = " + resp.Trailer.Get(k) + "\n")
	}

	// HTTP TLS ConnectionState
	buffer.WriteString("\nHTTP TLS ConnectionState:\n")
	if resp.TLS != nil {
		buffer.WriteString("HandshakeComplete:\n")
		buffer.WriteString("DidResume:\n")
		buffer.WriteString("NegotiatedProtocol: " + resp.TLS.NegotiatedProtocol + "\n")
		buffer.WriteString("NegotiatedProtocolIsMutual:\n")
		buffer.WriteString("ServerName: " + resp.TLS.ServerName)
	}

	// HTTP Body
	buffer.WriteString("\nHTTP BODY:\n")
	b, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "Error on ioutil.ReadAll(resp.Body):\n %v\n", err2)
		os.Exit(3)
	}
	_, err3 := buffer.Write(b)
	if err3 != nil {
		fmt.Fprintf(os.Stderr, "Error on buffer.Write(b):\n %v\n", err3)
		os.Exit(4)
	}

	textBytes := buffer.Bytes()
	ioutil.WriteFile("/libs/logs/tFetchHttp.log", textBytes, 0222)

	resp.Body.Close()
}
