package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go/clients/horizon"
)

const satoshipay = "https://stellar-horizon.satoshipay.io"
const sdf = "https://horizon.stellar.org"

type element struct {
	tx     string
	ledger int
	hash   string
}

func main() {
	setLogFile()
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
	for i, el := range txs {
		if i >= 1 {
			return
		}
		time.Sleep(time.Second / 20)

		log.Printf("submitting to network: ledger=%d, hash=%s, tx=%s", el.ledger, el.hash, el.tx)
		// postResp, e := hSDF.SubmitTransaction(el.tx)
		// if e != nil {
		// 	log.Printf("error: %s", e)
		// 	continue
		// }

		// log.Printf("result: %v", postResp)
	}
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
