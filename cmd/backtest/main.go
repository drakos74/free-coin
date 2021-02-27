package main

import (
	"context"
	"fmt"
)

func main() {

	ctx := context.Background()
	srv := New()
	go func() {
		err := srv.Run()
		if err != nil {
			panic(err.Error())
		}
	}()

	<-ctx.Done()
	fmt.Println(fmt.Sprintf("shutting down = %+v", srv))
}
