package main

import (
	"fmt"
	"io"
	"os"
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

func Test_FilePosition(t *testing.T) {
	f, err := os.OpenFile("test.test", os.O_CREATE, 0600)
	if err != nil {
		t.Error(err)
	}

	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(offset)
	f.Close()
	os.Remove("test.test")
}
