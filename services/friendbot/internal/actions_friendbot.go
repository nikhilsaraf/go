package internal

import (
	"net/http"

	client "github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/services/horizon/actions"
	"github.com/stellar/go/services/horizon/render/hal"
	"github.com/stellar/go/services/horizon/render/problem"
	"github.com/zenazn/goji/web"
)

// FriendbotAction causes an account at `Address` to be created.
type FriendbotAction struct {
	Friendbot *Bot
	actions.Base
	Address string
	Result  client.TransactionSuccess
}

// JSON is a method for actions.JSON
func (action *FriendbotAction) JSON() {
	action.Do(
		action.checkEnabled,
		action.loadAddress,
		action.loadResult,
		func() {
			hal.Render(action.W, action.Result)
		})
}

func (action *FriendbotAction) checkEnabled() {
	if action.Friendbot != nil {
		return
	}

	action.Err = &problem.P{
		Type:   "friendbot_disabled",
		Title:  "Friendbot is disabled",
		Status: http.StatusForbidden,
		Detail: "This horizon server is not configured to provide a friendbot. " +
			"Contact the server administrator if you believe this to be in error.",
	}
}

func (action *FriendbotAction) loadAddress() {
	action.Address = action.GetAddress("addr")
}

func (action *FriendbotAction) loadResult() {
	action.Result, action.Err = action.Friendbot.Pay(action.Address)
}

// ServeHTTPC implements Action for FriendbotAction.
func (action FriendbotAction) ServeHTTPC(c web.C, w http.ResponseWriter, r *http.Request) {
	ap := &action.Base
	ap.Prepare(c, w, r)
	ap.Execute(&action)
}
