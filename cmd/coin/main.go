package main

import (
	"context"
)

func main() {

	ctx, _ := context.WithCancel(context.Background())

	// this is a long running task ... lets keep the main thread occupied
	<-ctx.Done()

}
