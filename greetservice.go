package main

type GreetService struct{}

func (g *GreetService) Greet(name string) string {
	return "急急如律令 " + name + "!"
}
