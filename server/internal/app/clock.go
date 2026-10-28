package app

import (
	"sync"
	"time"
	_ "time/tzdata"

	"librehold/internal/game"
)

var worldZone = sync.OnceValue(func() *time.Location {
	loc, err := time.LoadLocation(game.WorldTimezone)
	if err != nil {
		panic(err)
	}
	return loc
})
