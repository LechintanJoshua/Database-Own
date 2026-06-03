package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd.AddCommand(insertCmd)
	rootCmd.AddCommand(getRowCmd)
	rootCmd.AddCommand(createTableCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(rangeCmd)

	rootCmd.PersistentFlags().StringVarP(&dbPath, "db", "d", "data.db", "Calea catre fisierul bazei de date")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
