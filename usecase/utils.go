package usecase

import (
	"time"
)

func (suite *UseCaseSuite) Wait(seconds int64) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
