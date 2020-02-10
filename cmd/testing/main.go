package main

import (
	"fmt"
	"github.com/h44z/eulumies"
)

func main() {
	eulumdat, err := eulumies.NewEulumdat("test/sample2.ldt", false)
	if err != nil {
		fmt.Println("Error parsing ldt:", err)
	} else {
		fmt.Println("Parsed LDT:", eulumdat.CompanyIdentification)
		err = eulumdat.Export("test/out.ldt")
		if err != nil {
			fmt.Println(err)
		}
	}
}
