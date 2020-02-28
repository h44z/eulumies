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

	ies, err := eulumies.NewIES("test/sample.ies", false)
	if err != nil {
		fmt.Println("Error parsing ies:", err)
	} else {
		fmt.Println("Parsed ies:", ies.Keywords["LUMINAIRE"])
		err = ies.Export("test/out.ies")
		if err != nil {
			fmt.Println(err)
		}
	}
}
