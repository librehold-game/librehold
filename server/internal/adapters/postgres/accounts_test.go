package postgres_test

import (
	"testing"

	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
)

func TestAccountStore(t *testing.T) {
	apptest.RunAccountStoreContract(t, func(t *testing.T) app.AccountStore { return postgres.NewAccounts(pgtest.New(t)) })
}
