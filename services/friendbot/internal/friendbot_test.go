package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sync"
)

// REGRESSION:  ensure that we can craft a transaction
func TestFriendbot_makeTx(t *testing.T) {
	fb := &Bot{
		Secret:          "SAQWC7EPIYF3XGILYVJM4LVAVSLZKT27CTEI3AFBHU2VRCMQ3P3INPG5",
		Network:         "Test SDF Network ; September 2015",
		StartingBalance: "100.00",
		sequence:        2,
	}

	_, err := fb.makeTx("GDJIN6W6PLTPKLLM57UW65ZH4BITUXUMYQHIMAZFYXF45PZVAWDBI77Z")
	assert.NoError(t, err)

	// ensure we're race free. NOTE:  presently, gb can't
	// run with -race on... we'll confirm this works when
	// horizon is in the monorepo
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, err := fb.makeTx("GDJIN6W6PLTPKLLM57UW65ZH4BITUXUMYQHIMAZFYXF45PZVAWDBI77Z")
		assert.NoError(t, err)
		wg.Done()
	}()
	go func() {
		_, err := fb.makeTx("GDJIN6W6PLTPKLLM57UW65ZH4BITUXUMYQHIMAZFYXF45PZVAWDBI77Z")
		assert.NoError(t, err)
		wg.Done()
	}()
	wg.Wait()
}
