package internal

import (
	"net/http"

	client "github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/support/render/hal"
	"github.com/stellar/go/support/render/problem"
)

// FriendbotHandler causes an account at `Address` to be created.
type FriendbotHandler struct {
	Friendbot *Bot
}

// Handle is a method that implements http.HandlerFunc
func (handler *FriendbotHandler) Handle(w http.ResponseWriter, r *http.Request) {
	result, err := handler.doHandle(r)
	if err != nil {
		problem.Render(log.DefaultLogger, w, err)
		return
	}

	hal.Render(w, result)
}

// doHandle is just a convenience method that returns the object to be rendered
func (handler *FriendbotHandler) doHandle(r *http.Request) (interface{}, error) {
	err := handler.checkEnabled()
	if err != nil {
		return nil, err
	}

	err = r.ParseForm()
	if err != nil {
		return nil, err
	}

	address, err := handler.loadAddress(r)
	if err != nil {
		return nil, err
	}

	return handler.loadResult(address)
}

func (handler *FriendbotHandler) checkEnabled() error {
	if handler.Friendbot != nil {
		return nil
	}

	return &problem.P{
		Type:   "friendbot_disabled",
		Title:  "Friendbot is disabled",
		Status: http.StatusForbidden,
		Detail: "Friendbot is disabled on this network. Contact the server administrator if you believe this to be in error.",
	}
}

func (handler *FriendbotHandler) loadAddress(r *http.Request) (string, error) {
	address := r.Form.Get("addr")
	_, err := strkey.Decode(strkey.VersionByteAccountID, address)
	return address, err
}

func (handler *FriendbotHandler) loadResult(address string) (client.TransactionSuccess, error) {
	return handler.Friendbot.Pay(address)
}
