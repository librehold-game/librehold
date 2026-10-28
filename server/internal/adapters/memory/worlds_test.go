package memory_test

import (
	"testing"

	"librehold/internal/adapters/memory"
	"librehold/internal/app/apptest"
)

func TestWorldStore(t *testing.T) {
	apptest.RunWorldStoreContract(t, func(t *testing.T) apptest.WorldBackend {
		s := memory.NewWorlds()
		return apptest.WorldBackend{Store: s, Updates: s.Updates(), Wakeups: s.Wakeups()}
	})
}
