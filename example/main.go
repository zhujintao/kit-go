package main

import (
	"context"
	"fmt"

	"github.com/zhujintao/kit-go/container"
)

func main() {
	cxt := context.Background()
	cli, err := container.NewClient(cxt)
	if err != nil {
		fmt.Println(err)
		return
	}
	cli.Create("zhujintao/base", "haha")
}
