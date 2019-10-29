package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/stellar/go/exp/hubble/elk_spike/data"
)

const myIndex = "my_first_index"
const shouldDelete = false
const shouldInsert = true
const shouldLog = false
const numDaysData = 365
const insertEveryN = 5000

var urlBase = fmt.Sprintf("http://localhost:9200/%s", myIndex)

func put(url string, contentType string, body io.Reader) (resp *http.Response, e error) {
	req, e := http.NewRequest("PUT", url, body)
	if e != nil {
		return nil, e
	}

	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func delete(url string) (resp *http.Response, e error) {
	req, e := http.NewRequest("DELETE", url, nil)
	if e != nil {
		return nil, e
	}

	return http.DefaultClient.Do(req)
}

func main() {
	now := time.Now()
	t := time.Now().Add(-numDaysData * 24 * time.Hour)

	firstNames := data.FirstNames()
	lastNames := data.LastNames()
	places := data.Places()

	r := rand.New(rand.NewSource(now.Unix()))
	r.Shuffle(len(firstNames), func(i, j int) { firstNames[i], firstNames[j] = firstNames[j], firstNames[i] })
	r.Shuffle(len(lastNames), func(i, j int) { lastNames[i], lastNames[j] = lastNames[j], lastNames[i] })
	r.Shuffle(len(places), func(i, j int) { places[i], places[j] = places[j], places[i] })

	autoidx := 0
	fni := 0
	lni := 0
	pi := 0

	firstInsertID := 0
	docs := []map[string]interface{}{}
	start := time.Now()
	for t.Before(now) {
		// generate value from 1000 to 9999
		value := 1000 + (rand.Float64() * 9999)
		body := map[string]interface{}{
			"first_name": firstNames[fni],
			"last_name":  lastNames[lni],
			"place":      places[pi],
			"timeMillis": t.Unix(),
			"timeString": t.String(),
			"time":       t,
			"value":      value,
		}
		docs = append(docs, body)

		if shouldDelete {
			respDelete, e := delete(urlBase + fmt.Sprintf("/_doc/%d", autoidx))
			if e != nil {
				log.Fatal(e)
			}
			defer respDelete.Body.Close()
			respDeleteBody, e := ioutil.ReadAll(respDelete.Body)
			if e != nil {
				log.Fatal(e)
			}
			if shouldLog {
				fmt.Printf("deleted: %s\n", respDeleteBody)
			}
		}

		// increment values for creation of next doc
		fni = (fni + 1) % len(firstNames)
		lni = (lni + 1) % len(lastNames)
		pi = (pi + 1) % len(places)
		t = t.Add(time.Duration(r.Intn(10000)) * time.Second) // close to 6k entries in 1 year
		autoidx++

		// only insert if it hits the bulk limit
		if autoidx%insertEveryN != 0 {
			continue
		}

		if shouldInsert {
			insertDocs(docs, firstInsertID)
		}
		// reset docs for next batch
		docs = []map[string]interface{}{}
		firstInsertID = autoidx
	}
	if shouldInsert {
		insertDocs(docs, firstInsertID)
	}

	elapsed := time.Now().Sub(start)
	log.Printf("number of entries inserted: %d", autoidx)
	log.Printf("time elapsed: %s", elapsed)
	log.Printf("inserted entries / second = %f", float64(autoidx)/elapsed.Seconds())
}

func insertDocs(docs []map[string]interface{}, firstInsertID int) {
	payload := []byte{}
	for _, d := range docs {
		// body, e := json.Marshal(map[string]interface{}{"create": d})
		body, e := json.Marshal(d)
		if e != nil {
			log.Fatal(e)
		}

		payload = append(payload, []byte("{ \"index\": {} }")...)
		payload = append(payload, []byte("\n")...)
		payload = append(payload, body...)
		payload = append(payload, []byte("\n")...)
	}

	url := urlBase + "/_doc/_bulk"
	if shouldLog {
		fmt.Printf("Req (url=%s):\n%s\n", url, string(payload))
	}
	respPut, e := http.DefaultClient.Post(url, "application/json", bytes.NewBuffer(payload))
	if e != nil {
		log.Fatal(e)
	}
	defer respPut.Body.Close()
	respPutBody, e := ioutil.ReadAll(respPut.Body)
	if e != nil {
		log.Fatal(e)
	}
	if shouldLog {
		fmt.Printf("Resp: %s\n\n", respPutBody)
	}
}
