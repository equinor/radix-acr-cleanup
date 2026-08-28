package delaytick

import (
	"math/rand"
	"time"
)

// Original file from https://github.com/kubereboot/kured/blob/main/pkg/delaytick/delaytick.go, licensed Apache-2.0.

// New Tick regularly after an initial delay randomly distributed between d/2 and d + d/2
func New(s rand.Source, d time.Duration) <-chan time.Time {
	c := make(chan time.Time)

	go func() {
		random := rand.New(s)
		time.Sleep(time.Duration(float64(d)/2 + float64(d)*random.Float64()))
		c <- time.Now()
		for t := range time.Tick(d) {
			c <- t
		}
	}()

	return c
}
