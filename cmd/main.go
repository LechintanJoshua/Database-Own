package main

import (
	"database-own/internal"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var dbPath string

var rootCmd = &cobra.Command{
	Use:   "mydb",
	Short: "O baza de date relationala minimalista scrisa in GO",
	Long: `Aceasta este o implementare proprie a unui motor de baza de date 
folosind un B+Tree, MMap, un LRU Page Cache si un strat relational pentru tabele.`,
}

// insertCmd gestioneaza inserarea unui rand intreg intr-un tabel
var insertCmd = &cobra.Command{
	Use:   "insert [tabel] [col1=val1] [col2=val2]...",
	Short: "Insereaza un rand nou intr-o tabela",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tableName := args[0]

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			fmt.Printf("Eroare la deschiderea DB: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		rec := &internal.Record{}

		for _, arg := range args[1:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Format invalid pentru date: %s. Foloseste col=val\n", arg)
				return
			}
			colName := parts[0]
			colVal := parts[1]

			if valInt, err := strconv.ParseInt(colVal, 10, 64); err == nil {
				rec.AddInt64(colName, valInt)
			} else {
				rec.AddStr(colName, []byte(colVal))
			}
		}

		_, err = db.Insert(tableName, *rec)
		if err != nil {
			fmt.Printf("Eroare la inserare: %v\n", err)
			return
		}
	},
}

// getRowCmd extrage un rand dintr-un tabel pe baza cheii primare
var getRowCmd = &cobra.Command{
	Use:   "get-row [tabel] [col_primara=valoare]",
	Short: "Extrage un rand din baza de date",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tableName := args[0]

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			fmt.Printf("Eroare la deschiderea DB: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		rec := &internal.Record{}

		for _, arg := range args[1:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Format invalid: %s. Foloseste col=val\n", arg)
				return
			}
			colName := parts[0]
			colVal := parts[1]

			if valInt, err := strconv.ParseInt(colVal, 10, 64); err == nil {
				rec.AddInt64(colName, valInt)
			} else {
				rec.AddStr(colName, []byte(colVal))
			}
		}

		found, err := db.Get(tableName, rec)
		if err != nil {
			fmt.Printf("Eroare la cautare: %v\n", err)
			return
		}

		if !found {
			fmt.Println("Randul nu a fost gasit.")
			return
		}

		internal.PrintRecord(rec)
	},
}

// createTableCmd gestioneaza crearea schemei unei tabele noi
var createTableCmd = &cobra.Command{
	Use:   "create-table [tabel] [nr_chei_primare] [col1:tip] [col2:tip]...",
	Short: "Creaza o tabela noua in baza de date",
	Long: `Comanda create-table verifica existenta tabelei in baza de date si daca
	aceasta nu exista, va fi creata.
	Minim: nume_tabel, nr_chei, si macar o coloana`,
	Args: cobra.MinimumNArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		tableName := args[0]

		pkeysCount, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Numarul de chei primare trebuie sa fie un numar intreg.")
			return
		}

		tdef := &internal.TableDef{
			Name:  tableName,
			PKeys: pkeysCount,
		}

		for _, arg := range args[2:] {
			parts := strings.SplitN(arg, ":", 2)
			if len(parts) != 2 {
				fmt.Printf("Format invalid pentru coloana: %s. Foloseste nume:tip\n", arg)
				return
			}

			colName := parts[0]
			colTypeStr := strings.ToLower(parts[1])

			tdef.Cols = append(tdef.Cols, colName)

			switch colTypeStr {
			case "int64", "int":
				tdef.Types = append(tdef.Types, internal.TYPE_INT64)
			case "string", "bytes":
				tdef.Types = append(tdef.Types, internal.TYPE_BYTES)
			default:
				fmt.Printf("Tip de date necunoscut: %s. Foloseste 'int64' sau 'string'\n", colTypeStr)
				return
			}
		}

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			fmt.Printf("Eroare la deschiderea DB: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		if err := db.TableNew(tdef); err != nil {
			fmt.Printf("Eroare la crearea tabelei: %v\n", err)
			return
		}

		fmt.Printf("Tabela '%s' a fost creata cu succes!\n", tableName)
	},
}

// deleteCmd gestioneaza stergerea unui rand dintr-o tabela
var deleteCmd = &cobra.Command{
	Use:   "delete-row [tabel] [col_primara=valoare]",
	Short: "Sterge un rand dintr-o tabela",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tableName := args[0]

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			fmt.Printf("Eroare la deschiderea DB: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		rec := &internal.Record{}

		for _, arg := range args[1:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Format invalid: %s. Foloseste col=val\n", arg)
				return
			}

			colName := parts[0]
			colVal := parts[1]

			if valInt, err := strconv.ParseInt(colVal, 10, 64); err == nil {
				rec.AddInt64(colName, valInt)
			} else {
				rec.AddStr(colName, []byte(colVal))
			}
		}

		found, err := db.Delete(tableName, *rec)
		if err != nil {
			fmt.Printf("Eroare la stergere: %v\n", err)
			return
		}

		if found {
			fmt.Println("Randul a fost sters.")
			return
		}
	},
}

// updateCmd gestioneaza actualizarea unui rand din tabela
var updateCmd = &cobra.Command{
	Use:   "update-row [tabel] [col_primara=valoare] [col_de_modificat=valoare_noua]...",
	Short: "Actualizeaza un rand dintr-o tabela",
	Args:  cobra.MinimumNArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		tableName := args[0]
		primaryKeyArg := args[1]

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			fmt.Printf("Eroare la deschiderea DB: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		rec := &internal.Record{}
		pkParts := strings.SplitN(primaryKeyArg, "=", 2)
		if len(pkParts) != 2 {
			fmt.Println("Format invalid pentru cheia primara. Foloseste col=val")
			return
		}

		if valInt, err := strconv.ParseInt(pkParts[1], 10, 64); err == nil {
			rec.AddInt64(pkParts[0], valInt)
		} else {
			rec.AddStr(pkParts[0], []byte(pkParts[1]))
		}

		found, err := db.Get(tableName, rec)
		if err != nil {
			fmt.Printf("Eroare la cautare: %v\n", err)
			return
		}
		if !found {
			fmt.Println("Randul nu a fost gasit.")
			return
		}

		for _, arg := range args[2:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Format invalid pentru date: %s. Foloseste col=val\n", arg)
				return
			}
			colName := parts[0]
			colVal := parts[1]

			idx := -1
			for i, c := range rec.Cols {
				if c == colName {
					idx = i
					break
				}
			}

			if idx == -1 {
				fmt.Printf("Coloana '%s' nu exista in acest tabel.\n", colName)
				return
			}

			if valInt, err := strconv.ParseInt(colVal, 10, 64); err == nil {
				rec.Vals[idx] = internal.Value{Type: internal.TYPE_INT64, I64: valInt}
			} else {
				rec.Vals[idx] = internal.Value{Type: internal.TYPE_BYTES, Str: []byte(colVal)}
			}
		}

		_, err = db.Update(tableName, *rec)
		if err != nil {
			fmt.Printf("Eroare la actualizare: %v\n", err)
			return
		}

		fmt.Println("Randul a fost actualizat cu succes!")
	},
}

func main() {
	rootCmd.AddCommand(insertCmd)
	rootCmd.AddCommand(getRowCmd)
	rootCmd.AddCommand(createTableCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(updateCmd)

	rootCmd.PersistentFlags().StringVarP(&dbPath, "db", "d", "data.db", "Calea catre fisierul bazei de date")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
