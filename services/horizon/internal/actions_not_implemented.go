package horizon

import (
	"github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/support/log"
	sProblem "github.com/stellar/go/support/render/problem"
)

// NotImplementedAction renders a NotImplemented prblem
type NotImplementedAction struct {
	Action
}

// JSON is a method for actions.JSON
func (action *NotImplementedAction) JSON() {
	sProblem.Render(log.Ctx(action.Ctx), action.W, problem.NotImplemented)
}
