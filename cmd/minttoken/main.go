package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/auth"
)

func main() {
	dataDir := "./pb_data"
	if len(os.Args) > 1 {
		dataDir = os.Args[1]
	}
	pb, err := auth.New(dataDir, 24)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth.New: %v\n", err)
		os.Exit(1)
	}
	email := "testagent@ishos.com"
	if len(os.Args) > 2 {
		email = os.Args[2]
	}
	record, err := pb.FindEmployeeByEmail(email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find employee: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 3 {
		record.SetPassword(os.Args[3])
		if err := pb.App.Save(record); err != nil {
			fmt.Fprintf(os.Stderr, "save employee: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("password updated")
		return
	}

	token, err := pb.GenerateAuthToken(record, time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(token)
}
