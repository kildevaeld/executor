package main

import (
	"fmt"
	"time"

	"github.com/kildevaeld/dict"
	"github.com/kildevaeld/executor"
)

type Reply struct {
	Title string
	Date  time.Time
}

type Comply struct {
	Title string
	Reply Reply
	Tags  []string
}

type Test struct {
}

func (self *Test) Run(ctx *Comply, title string, reply *Reply) error {
	fmt.Printf("Hello: %s from Test\n", title)
	*reply = Reply{
		Title: "Hello",
		Date:  time.Now(),
	}
	return nil
}

func main() {
	e := executor.NewExecutor()

	e.Register("test", &Test{})

	comply := Comply{
		Title: "Hello",
		Reply: Reply{
			Title: "An othher title",
			Date:  time.Now(),
		},
		Tags: []string{"Hello", "World"},
	}

	bs, _ := executor.CallToJSON("test.Run", comply, "Test Mig") //json.Marshal(call)

	var out dict.Map
	err := e.CallWithJSON(bs, &out)
	if err != nil {
		fmt.Printf("Error %v", err)
	}
	fmt.Printf("%#v", out)
}
