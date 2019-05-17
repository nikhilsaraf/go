package io

import (
	"fmt"
	"io"
	"sync"

	"github.com/stellar/go/support/historyarchive"
	"github.com/stellar/go/xdr"
)

// MultiMergeStateReader is the 18-way merge implementation that reads HistoryArchiveState
type MultiMergeStateReader struct {
	has      *historyarchive.HistoryArchiveState
	archive  *historyarchive.Archive
	sequence uint32
	active   bool
	readChan chan readResult
	once     *sync.Once
}

// enforce MultiMergeStateReader to implement StateReader
var _ StateReader = &MultiMergeStateReader{}

// MakeMultiMergeStateReader is a factory method for MultiMergeStateReader
func MakeMultiMergeStateReader(archive *historyarchive.Archive, sequence uint32, bufferSize uint16) (*MultiMergeStateReader, error) {
	has, e := archive.GetCheckpointHAS(sequence)
	if e != nil {
		return nil, fmt.Errorf("unable to get checkpoint HAS at ledger sequence %d: %s", sequence, e)
	}

	return &MultiMergeStateReader{
		has:      &has,
		archive:  archive,
		sequence: sequence,
		active:   false,
		readChan: make(chan readResult, bufferSize),
		once:     &sync.Once{},
	}, nil
}

// BufferReads triggers the streaming logic needed to be done before Read() can actually produce a result
func (msr *MultiMergeStateReader) BufferReads() {
	msr.once.Do(msr.start)
}

func (msr *MultiMergeStateReader) start() {
	msr.active = true
	go msr.bufferNext()
}

func (msr *MultiMergeStateReader) bufferNext() {
	defer close(msr.readChan)

	// iterate from newest to oldest bucket and track keys already seen
	seen := map[xdr.LedgerKey]bool{}
	for _, hash := range msr.has.Buckets() {
		if !msr.archive.BucketExists(hash) {
			msr.readChan <- readResult{xdr.LedgerEntry{}, fmt.Errorf("bucket hash does not exist: %s", hash)}
			return
		}

		// read bucket detail
		filepathChan, errChan := msr.archive.ListBucket(historyarchive.HashPrefix(hash))

		// read from channels
		var filepath string
		var e error
		var ok bool
		select {
		case fp, okb := <-filepathChan:
			// example filepath: prd/core-testnet/core_testnet_001/bucket/be/3c/bf/bucket-be3cbfc2d7e4272c01a1a22084573a04dad96bf77aa7fc2be4ce2dec8777b4f9.xdr.gz
			filepath, e, ok = fp, nil, okb
		case err, okb := <-errChan:
			filepath, e, ok = "", err, okb
			// TODO do we need to do anything special if e is nil here?
		}
		if !ok {
			// move on to next bucket when this bucket is fully consumed or empty
			continue
		}

		// process values
		if e != nil {
			msr.readChan <- readResult{xdr.LedgerEntry{}, fmt.Errorf("received error on errChan when listing buckets for hash '%s': %s", hash, e)}
			return
		}

		bucketPath, e := getBucketPath(bucketRegex, filepath)
		if e != nil {
			msr.readChan <- readResult{xdr.LedgerEntry{}, fmt.Errorf("cannot get bucket path for filepath '%s' with hash '%s': %s", filepath, hash, e)}
			return
		}

		var shouldContinue bool
		seen, shouldContinue = msr.streamBucketContents(bucketPath, hash, seen)
		if !shouldContinue {
			return
		}
	}
}

// streamBucketContents pushes value onto the read channel, returning false when the channel needs to be closed otherwise true
func (msr *MultiMergeStateReader) streamBucketContents(
	bucketPath string,
	hash historyarchive.Hash,
	seen map[xdr.LedgerKey]bool,
) (map[xdr.LedgerKey]bool, bool) {
	rdr, e := msr.archive.GetXdrStream(bucketPath)
	if e != nil {
		msr.readChan <- readResult{xdr.LedgerEntry{}, fmt.Errorf("cannot get xdr stream for bucketPath '%s': %s", bucketPath, e)}
		return seen, false
	}
	defer rdr.Close()

	n := 0
	for {
		var entry xdr.BucketEntry
		if e = rdr.ReadOne(&entry); e != nil {
			if e == io.EOF {
				// proceed to the next bucket hash
				return seen, true
			}
			msr.readChan <- readResult{xdr.LedgerEntry{}, fmt.Errorf("Error on XDR record %d of bucketPath '%s': %s", n, bucketPath, e)}
			return seen, false
		}
		n++

		liveEntry, ok := entry.GetLiveEntry()
		if ok {
			// ignore entry if we've seen it previously
			key := liveEntry.LedgerKey()
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = true

			// since readChan is a buffered channel we block here until one item is consumed on the dequeue side.
			// this is our intended behavior, which ensures we only buffer exactly bufferSize results in the channel.
			msr.readChan <- readResult{liveEntry, nil}
		}
		// we can ignore dead entries because we're only ever concerned with the first live entry values
	}
}

// GetSequence impl.
func (msr *MultiMergeStateReader) GetSequence() uint32 {
	return msr.sequence
}

// Read returns a new ledger entry on each call, returning false when the stream ends
func (msr *MultiMergeStateReader) Read() (bool, xdr.LedgerEntry, error) {
	if !msr.active {
		msr.BufferReads()
	}

	// blocking call. anytime we consume from this channel, the background goroutine will stream in the next value
	result, ok := <-msr.readChan
	if !ok {
		// when channel is closed then return false with empty values
		return false, xdr.LedgerEntry{}, nil
	}

	if result.e != nil {
		return true, xdr.LedgerEntry{}, fmt.Errorf("error while reading from background channel: %s", result.e)
	}
	return true, result.entry, nil
}
