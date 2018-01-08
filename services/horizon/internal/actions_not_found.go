package horizon

import (
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/support/render/problem"
)

// NotFoundAction renders a 404 response
type NotFoundAction struct {
	Action
}

// JSON is a method for actions.JSON
func (action *NotFoundAction) JSON() {
	problem.Render(log.Ctx(action.Ctx), action.W, problem.NotFound)
}
