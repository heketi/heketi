package main

import (
	"flag"
	"fmt"
	"github.com/heketi/heketi/client/go/utils"
)

type Cmd struct {
	name  string
	flags *flag.FlagSet
}

// Arith ----------------------------------------------------
type ArithCommand struct {
	// Generic stuff.  This is called
	// embedding.  In other words, the members in
	// the struct below are here also
	Cmd

	// Now we can add stuff that specific to this
	// structure
	operation string
}

func NewArithCommand() *ArithCommand {
	cmd := &ArithCommand{}
	cmd.name = "arith"

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.StringVar(&cmd.operation, "op", "a", "help message")
	cmd.flags.Usage = func() {
		fmt.Println("Hello from my usage")
	}

	return cmd
}

func (a *ArithCommand) Name() string {
	return a.name

}

func (a *ArithCommand) Parse(args []string) error {
	return a.flags.Parse(args)
}

func (a *ArithCommand) add() int {
	args := a.flags.Args()[1 : len(a.flags.Args())-1]
	fmt.Println(args)
	sum := 0

	//convert string arr to int arr
	ret := utils.StrArrToIntArr(args)
	//sum all numbers
	for _, num := range ret {
		sum = sum + num
	}
	fmt.Println(sum)
	return sum
}

func (a *ArithCommand) subtract() int {
	if len(a.flags.Args()) > 3 {
		panic("Oops, I can only subtract 2 numbers!")
	}
	args := a.flags.Args()[1:3]

	ret := utils.StrArrToIntArr(args)

	difference := ret[0] - ret[1]
	fmt.Println(difference)
	return difference

}

func (a *ArithCommand) Do() error {

	fmt.Println(a.flags.Args())
	switch a.flags.Arg(0) {
	case "add":
		// if a.operation != "a" {
		a.add()
		// }
	case "subtract":
		a.subtract()
	default:
		fmt.Println("NAH")
	}
	fmt.Println(a.flags.Arg(1))
	fmt.Println("Options:")
	fmt.Println(a.operation)

	return nil
}

// Echo ----------------------------------------------------
type EchoCommand struct {
	// Generic stuff.  This is called
	// embedding.  In other words, the values in
	// the struct below are here also
	Cmd

	// it echoes anything after the command
	strings []string
}

func NewEchoCommand() *EchoCommand {
	cmd := &EchoCommand{}
	cmd.name = "echo"

	cmd.flags = flag.NewFlagSet(cmd.name, flag.ExitOnError)
	cmd.flags.Usage = func() {
		fmt.Println("Hello from my usage")
	}

	return cmd
}

func (e *EchoCommand) Name() string {
	return e.name

}

func (e *EchoCommand) Parse(args []string) error {
	err := e.flags.Parse(args)
	if err != nil {
		return err
	}
	e.strings = e.flags.Args()

	return nil
}

func (e *EchoCommand) Do() error {
	fmt.Println("Echo")
	fmt.Println(e.strings)
	return nil
}

// --- Use Interface for common structure access

type Command interface {
	Name() string
	Parse([]string) error
	Do() error
}

type Commands []Command

// ------ Main
func main() {
	flag.Parse()
	commands := Commands{
		NewArithCommand(),
		NewEchoCommand(),
	}

	for _, cmd := range commands {
		if flag.Arg(0) == cmd.Name() {
			cmd.Parse(flag.Args()[1:])
			cmd.Do()
			return
		}
	}

	fmt.Println("Command not found")
}
