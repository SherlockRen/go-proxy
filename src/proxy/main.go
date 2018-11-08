package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type reqStruct struct {
	R *http.Request
}

type resStruct struct {
	Header map[string][]string
	Cookie []*http.Cookie
	Result []byte
	Status int
}

type response struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    []byte `json:"data"`
}

var (
	workNum     uint32
	MAX_REQUEST int
	LISTEN_PORT string
	REQ_TIMEOUT time.Duration
	workChan    = make(chan reqStruct)
	respChan    = make(chan resStruct)
)

func init() {
	max := os.Getenv("MAX_REQUEST")
	timeout := os.Getenv("REQ_TIMEOUT")
	LISTEN_PORT = os.Getenv("LISTEN_PORT")
	if max == "" {
		max = "2"
	}
	if timeout == "" {
		timeout = "300"
	}
	if LISTEN_PORT == "" {
		LISTEN_PORT = ":8080"
	}
	MAX_REQUEST, err := strconv.Atoi(max)
	REQ_TIMEOUT, terr := strconv.Atoi(timeout)
	if err != nil {
		log.Fatalf("INIT ERROR: set max request failed (%s)", err)
	}
	if terr != nil {
		log.Fatalf("INIT ERROR: set timeout failed (%s)", terr)
	}

	fmt.Printf("NOTICE:PORT(%s), MAX(%d), TIMEOUT(%d)\n", LISTEN_PORT, MAX_REQUEST, REQ_TIMEOUT)

	workChan = make(chan reqStruct, MAX_REQUEST)
	respChan = make(chan resStruct, MAX_REQUEST)
	atomic.AddUint32(&workNum, uint32(MAX_REQUEST))

	for i := 0; i < MAX_REQUEST; i++ {
		go work(i)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	log.Fatal(http.ListenAndServe(LISTEN_PORT, mux))
}

func handle(w http.ResponseWriter, r *http.Request) {
	request := reqStruct{r}
	var res = response{http.StatusOK, "", []byte("")}
	// check workNum
	fmt.Printf("NOTICE: NUM(%d) \n", workNum)
	if atomic.LoadUint32(&workNum) <= 0 {
		res.Status = http.StatusLocked
		res.Message = "Server is buzy!!!"
		fmt.Printf("RUNTIME ERROR: server buzy max(%d) \n", MAX_REQUEST)
		resJson, _ := json.Marshal(res)
		w.Write([]byte(resJson))
		return
	}

	// send to work
	workChan <- request
	// check wait
	if wait := r.Header.Get("NO-WAIT"); wait == "1" {
		resJson, _ := json.Marshal(res)
		w.Write([]byte(resJson))
		return
	}
	// accept response
	resp := <-respChan
	// set header
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	// set cookie
	for _, value := range resp.Cookie {
		w.Header().Add(value.Name, value.Value)
	}

	// set status
	w.WriteHeader(resp.Status)
	_, err := w.Write(resp.Result)
	if err != nil {
		fmt.Printf("RUNTIME ERROR: failed to write respost(%s)\n", err)
	}
}

func work(id int) {
	for {
		select {
		case p := <-workChan:
			doLock("cut")
			doRequest(id, p)
			doLock("add")
		}
	}
}

func doRequest(id int, p reqStruct) {
	r := p.R
	proxyUrl, err := url.QueryUnescape(r.FormValue("url"))
	if proxyUrl == "" || err != nil {
		fmt.Println("RUNTIME ERROR: Invalid realUrl")
		respChan <- resStruct{Result: []byte("Invalid url"), Status: http.StatusBadRequest}
		return
	}
	// new form body
	var newBody url.Values = r.Form
	delete(r.Form, "url")
	// retry
	for i := 0; i < 3; i++ {
		// create request
		req, err := http.NewRequest(r.Method, proxyUrl, strings.NewReader(newBody.Encode()))
		if err != nil {
			fmt.Printf("RUNTIME ERROR: NewRequest(%s)\n", err)
			continue
		}
		// set header
		for k, v := range r.Header {
			if k == "NO-WAIT" {
				continue
			}
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
		// set cookie
		for _, value := range r.Cookies() {
			req.Header.Add(value.Name, value.Value)
		}

		// do request
		client := &http.Client{
			Timeout: (REQ_TIMEOUT * time.Millisecond),
		}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("RUNTIME ERROR: client Do(%s)\n", err)
			continue
		}
		// read result
		result, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("RUNTIME ERROR: read body(%s)\n", err)
			continue
		}
		defer resp.Body.Close()
		if wait := r.Header.Get("NO-WAIT"); wait == "" {
			respChan <- resStruct{resp.Header, resp.Request.Cookies(), result, resp.StatusCode}
		}

		break
	}
}

func doLock(action string) {
	if action == "cut" {
		delta := int32(-1)
		atomic.AddUint32(&workNum, uint32(delta))
	} else {
		atomic.AddUint32(&workNum, uint32(1))
	}
}
