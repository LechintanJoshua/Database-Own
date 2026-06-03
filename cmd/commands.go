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

// parseNameAndOpenFile obtine numele tabelei si deschide fisierul
// in care se afla datele
func parseNameAndOpenFile(args []string) (string, *internal.DB) {
	tableName := args[0]

	db, err := internal.OpenDB(dbPath)
	if err != nil {
		fmt.Printf("Eroare la deschiderea DB: %v\n", err)
		os.Exit(1)
	}

	return tableName, db
}

// addArgsToRecord preia datele date de pe linia de comanda
// si le adauga intr-un rand, returnand o eroare daca formatul e gresit
func addArgsToRecord(args []string, pos int, rec *internal.Record) error {
	for _, arg := range args[pos:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("format invalid pentru date: %s. Foloseste col=val", arg)
		}
		colName := parts[0]
		colVal := parts[1]

		if valInt, err := strconv.ParseInt(colVal, 10, 64); err == nil {
			rec.AddInt64(colName, valInt)
		} else {
			rec.AddStr(colName, []byte(colVal))
		}
	}
	return nil
}

// addTableSchemaArgs adauga definitia tabelei care va fi creata in variabila
// si verifica daca formatul respecta comanda
func addTableSchemaArgs(args []string, pos int, tdef *internal.TableDef) error {
	for _, arg := range args[pos:] {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("format invalid pentru coloana: %s. Foloseste nume:tip", arg)
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
			return fmt.Errorf("Tip de date necunoscut: %s. Foloseste 'int64' sau 'string'", colTypeStr)
		}
	}

	return nil
}

// addUpdateToRecord ordoneaza noile datele in rand si verifica daca acestea exista
func addUpdatesToRecord(args []string, pos int, rec *internal.Record) error {
	for _, arg := range args[pos:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("Format invalid pentru date: %s. Foloseste col=val", arg)
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
			return fmt.Errorf("Coloana '%s' nu exista in acest tabel.", colName)
		}

		if valInt, err := strconv.ParseInt(colVal, 10, 64); err == nil {
			rec.Vals[idx] = internal.Value{Type: internal.TYPE_INT64, I64: valInt}
		} else {
			rec.Vals[idx] = internal.Value{Type: internal.TYPE_BYTES, Str: []byte(colVal)}
		}
	}

	return nil
}

// checkLimAndPrintRows verifica limita obtinuta si afiseaza toate
// randurile pana la acea limita din tabela. Daca limita < 0 se opreste,
// daca nu este data ca argument se presupune toata tabela
func checkLimAndPrintRows(tableName string, db *internal.DB, args []string) {
	limit := 0
	if len(args) > 1 {
		var err error
		limit, err = strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Limita trebuie sa fie un numar intreg valid.")
			return
		}
	}

	sc, err := db.ScanAll(tableName)
	if err != nil {
		fmt.Printf("Eroare la initializare scanner: %v\n", err)
		return
	}

	internal.PrintTableResults(tableName, sc, limit)
}

// insertCmd gestioneaza inserarea unui rand intreg intr-un tabel
var insertCmd = &cobra.Command{
	Use:   "insert [tabel] [col1=val1] [col2=val2]...",
	Short: "Insereaza un rand nou intr-o tabela",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tableName, db := parseNameAndOpenFile(args)
		defer db.Close()
		rec := &internal.Record{}

		if err := addArgsToRecord(args, 1, rec); err != nil {
			fmt.Println(err)
			return
		}

		if _, err := db.Insert(tableName, *rec); err != nil {
			fmt.Printf("Eroare la inserare: %v\n", err)
			return
		}

		fmt.Println("Randul a fost inserat cu succes.")
	},
}

// getRowCmd extrage un rand dintr-un tabel pe baza cheii primare
var getRowCmd = &cobra.Command{
	Use:   "get-row [tabel] [col_primara=valoare]",
	Short: "Extrage un rand din baza de date",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tableName, db := parseNameAndOpenFile(args)
		defer db.Close()
		rec := &internal.Record{}

		if err := addArgsToRecord(args, 1, rec); err != nil {
			fmt.Println(err)
			return
		}

		if found, err := db.Get(tableName, rec); err != nil {
			fmt.Printf("Eroare la cautare: %v\n", err)
			return
		} else if !found {
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
		tableName, db := parseNameAndOpenFile(args)
		defer db.Close()

		pkeysCount, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Numarul de chei primare trebuie sa fie un numar intreg.")
			return
		}

		tdef := &internal.TableDef{
			Name:  tableName,
			PKeys: pkeysCount,
		}

		if err := addTableSchemaArgs(args, 2, tdef); err != nil {
			fmt.Println(err)
			return
		}

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
		tableName, db := parseNameAndOpenFile(args)
		defer db.Close()

		rec := &internal.Record{}
		if err := addArgsToRecord(args, 1, rec); err != nil {
			fmt.Println(err)
			return
		}

		if found, err := db.Delete(tableName, *rec); err != nil {
			fmt.Printf("Eroare la stergere: %v\n", err)
			return
		} else if found {
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
		tableName, db := parseNameAndOpenFile(args)
		primaryKeyArg := args[1]
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

		if found, err := db.Get(tableName, rec); err != nil {
			fmt.Printf("Eroare la cautare: %v\n", err)
			return
		} else if !found {
			fmt.Println("Randul nu a fost gasit.")
			return
		}

		if err := addUpdatesToRecord(args, 2, rec); err != nil {
			fmt.Println(err)
			return
		}

		if _, err := db.Update(tableName, *rec); err != nil {
			fmt.Printf("Eroare la actualizare: %v\n", err)
			return
		}

		fmt.Println("Randul a fost actualizat cu succes!")
	},
}

var listCmd = &cobra.Command{
	Use:   "list-table [tabel] [limita]",
	Short: "Afiseaza randurile dintr-o tabela pana la o limita data",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tableName, db := parseNameAndOpenFile(args)
		defer db.Close()

		checkLimAndPrintRows(tableName, db, args)
	},
}
