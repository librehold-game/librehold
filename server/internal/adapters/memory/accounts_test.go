package memory_test

import (
	"testing"

	"librehold/internal/adapters/memory"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
)

func TestAccountStore(t *testing.T) {
	apptest.RunAccountStoreContract(t, func(t *testing.T) app.AccountStore { return memory.NewAccounts() })
}
