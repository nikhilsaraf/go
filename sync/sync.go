package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/errors"
)

const satoshipay = "https://stellar-horizon.satoshipay.io"
const sdf = "https://horizon.stellar.org"

const perS = 15
const mode = "resubmit" // or "submit" or "resubmit"

type element struct {
	tx     string
	ledger int
	hash   string
}

func main() {
	setLogFile()

	log.Printf("configured to submit %d tx per second using mode = '%s'", perS, mode)

	const firstLedger = 23899050

	var e error
	// hSatoshipay := &horizon.Client{
	// 	URL:  satoshipay,
	// 	HTTP: http.DefaultClient,
	// }

	h := http.DefaultClient

	// resp, e := h.Get(satoshipay + "/transactions?order=desc")
	// if e != nil {
	// 	panic(e)
	// }

	// defer resp.Body.Close()
	// b, e := ioutil.ReadAll(resp.Body)
	// if e != nil {
	// 	panic(e)
	// }

	txRev := []element{}
	URL := satoshipay + "/transactions?order=desc"

	keepGoing := true
	for keepGoing {
		var output map[string]interface{}
		e = JSONRequest(h, "GET", URL, "", map[string]string{}, &output, "error")
		if e != nil {
			panic(e)
		}

		embed1 := output["_embedded"]
		embed2 := embed1.(map[string]interface{})
		records1 := embed2["records"]
		records2 := records1.([]interface{})
		for _, r1 := range records2 {
			r2 := r1.(map[string]interface{})
			// var ledger uint64 = r2["envelope_xdr"].(uint64)
			// var ledger uint64 = r2["ledger"].(uint64)
			// log.Printf("%d", ledger)

			envelope := r2["envelope_xdr"]
			// log.Printf("%v", envelope)
			envStr := envelope.(string)

			hash := r2["hash"].(string)

			links := r2["_links"].(map[string]interface{})
			ledgerStruct := links["ledger"].(map[string]interface{})
			ledgerURL := ledgerStruct["href"].(string)
			ledgerParts := strings.Split(ledgerURL, "/")
			ledger := ledgerParts[len(ledgerParts)-1]

			log.Printf("ledger=%s, hash=%s", ledger, hash)

			l, e := strconv.Atoi(ledger)
			if e != nil {
				panic(e)
			}

			txRev = append(txRev, element{
				tx:     envStr,
				ledger: l,
				hash:   hash,
			})

			if l < firstLedger {
				keepGoing = false
				break
			}
		}

		upLinks := output["_links"].(map[string]interface{})
		nextStruct := upLinks["next"].(map[string]interface{})
		nextHRef := nextStruct["href"].(string)
		log.Printf(nextHRef)

		URL = nextHRef
	}

	log.Println()
	log.Println()
	log.Println()

	txs := []element{}
	for i := len(txRev) - 1; i >= 0; i-- {
		el := txRev[i]
		txs = append(txs, el)
		// log.Printf("ledger=%d, hash=%s, tx=%s", el.ledger, el.hash, el.tx)
	}

	hSDF := &horizon.Client{
		URL:  sdf,
		HTTP: http.DefaultClient,
	}
	if mode == "submit" {
		for i, el := range txs {
			time.Sleep(time.Second / perS)
			submitTx(i, hSDF, el)
		}
	} else if mode == "resubmit" {
		for i, el := range txs {
			_, e := hSDF.LoadTransaction(el.hash)
			if e != nil {
				log.Printf("transaction not found on SDF, resubmitting (i = %d): ledger=%d, hash=%s", i, el.ledger, el.hash)

				submitTx(i, hSDF, el)
				time.Sleep(time.Second / perS)
				continue
			}
			log.Printf("(i = %d) found transaction on SDF with hash: %s", i, el.hash)
		}
	}
}

func submitTx(i int, hSDF *horizon.Client, el element) {
	defer log.Println()
	log.Printf("submitting to network (i = %d): ledger=%d, hash=%s, tx=%s", i, el.ledger, el.hash, el.tx)
	postResp, e := hSDF.SubmitTransaction(el.tx)
	if e != nil {
		if herr, ok := errors.Cause(e).(*horizon.Error); ok {
			var rcs *horizon.TransactionResultCodes
			rcs, e = herr.ResultCodes()
			if e != nil {
				log.Printf("error: no result codes from horizon: %s\n", e)
				return
			}

			if rcs.TransactionCode == "tx_bad_seq" {
				log.Println("error: tx_bad_seq")
				return
			}

			log.Println("error: result code details: tx code = ", rcs.TransactionCode, ", opcodes = ", rcs.OperationCodes)
		} else {
			log.Printf("error: tx failed for unknown reason, error message: %s\n", e)
		}
		return
	}

	btes, e := json.MarshalIndent(postResp, "", "    ")
	if e != nil {
		log.Printf("error marshaling resp: %s", e)
		log.Printf("last result error: %v", postResp)
		return
	}
	log.Printf("tx result: %v", string(btes))
}

func setLogFile() {
	t := time.Now().Format("20060102T150405MST")
	fileName := fmt.Sprintf("sync_%s.log", t)

	f, e := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if e != nil {
		log.Fatalf("failed to set log file: %s", e)
		return
	}

	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)

	log.Printf("logging to file: %s\n", fileName)
	// we want to create a deferred recovery function here that will log panics to the log file and then exit
	defer logPanic()
}

func logPanic() {
	if r := recover(); r != nil {
		st := debug.Stack()
		log.Printf("PANIC!! recovered to log it in the file\npanic: %v\n\n%s\n", r, string(st))
	}
}

// JSONRequest submits an HTTP web request and parses the response into the responseData object as JSON
func JSONRequest(
	httpClient *http.Client,
	method string,
	reqURL string,
	data string,
	headers map[string]string,
	responseData interface{}, // the passed in responseData should be a pointer
	errorKey string,
) error {
	// create http request
	req, e := http.NewRequest(method, reqURL, strings.NewReader(data))
	if e != nil {
		return fmt.Errorf("could not create http request: %s", e)
	}

	// add headers
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	// execute request
	resp, e := httpClient.Do(req)
	if e != nil {
		return fmt.Errorf("could not execute http request: %s", e)
	}
	defer resp.Body.Close()

	// read response
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		return fmt.Errorf("could not read http response: %s", e)
	}
	bodyString := string(body)

	// ensure Content-Type is json
	contentType, _, e := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if e != nil {
		return fmt.Errorf("could not read 'Content-Type' header in http response: %s | response body: %s", e, bodyString)
	}
	if contentType != "application/json" && contentType != "application/hal+json" {
		return fmt.Errorf("invalid 'Content-Type' header in http response ('%s'), expecting 'application/json' or 'application/hal+json', response body: %s", contentType, bodyString)
	}

	if errorKey != "" {
		var errorResponse interface{}
		e = json.Unmarshal(body, &errorResponse)
		if e != nil {
			return fmt.Errorf("could not unmarshall response body to check for an error response: %s | bodyString: %s", e, bodyString)
		}

		switch er := errorResponse.(type) {
		case map[string]interface{}:
			if _, ok := er[errorKey]; ok {
				return fmt.Errorf("error in response, bodyString: %s", bodyString)
			}
		}
	}

	if responseData != nil {
		// parse response, the passed in responseData should be a pointer
		e = json.Unmarshal(body, responseData)
		if e != nil {
			return fmt.Errorf("could not unmarshall response body into json: %s | response body: %s", e, bodyString)
		}
	}

	return nil
}