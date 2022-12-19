package main

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func printer(id int, notifChan chan struct{}, wg *sync.WaitGroup) {
	<-notifChan
	fmt.Println(fmt.Sprintf("ID: %d is done", id))
	wg.Done()
}

func TestRoutineChanLogic(t *testing.T) {
	n := 5
	notifChan := make(chan struct{}, n)
	wg := &sync.WaitGroup{}
	wg.Add(n)

	tick := time.NewTicker(time.Second)

	for i := 0; i < n; i++ {
		select {
		case <-tick.C:
			notifChan <- struct{}{}
			fmt.Println(fmt.Sprintf("Tick %d", i))
			break
		}
	}

	for i := 0; i < n; i++ {
		go printer(i, notifChan, wg)
	}

	wg.Wait()
	fmt.Println("Done")
}
