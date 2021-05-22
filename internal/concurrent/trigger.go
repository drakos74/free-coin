package concurrent

import (
	"sync"
)

func Async(exec func()) {
	var mutex = new(sync.Mutex)
	mutex.Lock()
	go func() {
		mutex.Unlock()
		exec()
	}()
	// make sure we wait for the go routine to initialise.
	// before we proceed
	mutex.Lock()
}
