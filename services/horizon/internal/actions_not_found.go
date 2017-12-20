package horizon

import (
	"github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/support/log"
	sProblem "github.com/stellar/go/support/render/problem"
)

// NotFoundAction renders a 404 response
type NotFoundAction struct {
	Action
}

// JSON is a method for actions.JSON
func (action *NotFoundAction) JSON() {
	sProblem.Render(log.Ctx(action.Ctx), action.W, problem.NotFound)
}
