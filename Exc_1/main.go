package main

import "fmt"

type Dog struct {
	Name string
}

func (d Dog) bark() {
	fmt.Printf("Bark! My name is %s!\n", d.Name)
}

func main() {
	var dog1 Dog = Dog{}

	dog1.Name = "Jimmy"

	dog1.bark()
}
