// Package post holds the contract test for POST /users/{id}/cards.
package post

import (
	"github.com/bulkashmak/pesto"
	"github.com/bulkashmak/pesto/contract"
)

func TestContract(t *pesto.T) {
	contract.Run(t, "contract.yaml")
}
