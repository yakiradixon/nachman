package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/matoous/go-nanoid/v2"
	_ "github.com/mattn/go-sqlite3"
)

type Work struct {
	ID         string `json:"id"`
	Author     string `json:"author"`
	Title      string `json:"title"`
	ISBN       string `json:"isbn"`
	Source     string `json:"source"`
	FromImport bool
}

type ImportedWork struct {
	ID     string `json:"books_id"`
	Author string `json:"primaryauthor"`
	Title  string `json:"title"`
	// ISBN     map[string]string `json:"isbn"`
	ISBN       string `json:"originalisbn"`
	Source     string `json:"source"`
	FromImport bool
}

var (
	catalogFile   = "catalog.json"
	importFile    = "import.json"
	importOnStart = false
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./works.db")
	if err != nil {
		log.Fatal(err)
	}
	createSchema()

	if importOnStart {
		importBooks()
	}

	http.HandleFunc("/", formHandler)
	http.HandleFunc("/work/", viewWorkHandler)
	http.HandleFunc("/work/new", newWorkHandler)
	http.HandleFunc("/work/create", createWorkHandler)
	http.HandleFunc("/work/edit/", editWorkHandler)
	http.HandleFunc("/work/update/", updateWorkHandler)
	http.HandleFunc("/work/delete/", deleteWorkHandler)
	http.HandleFunc("/export", exportHandler)
	http.HandleFunc("/search", searchHandler)

	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createSchema() {
	schema := `
	CREATE TABLE IF NOT EXISTS works (
		id TEXT PRIMARY KEY,
		author TEXT,
		title TEXT,
		isbn TEXT,
		source TEXT,
		from_import INTEGER
	);`
	_, err := db.Exec(schema)
	if err != nil {
		log.Fatal(err)
	}
}

func formHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, author, title, isbn, source, from_import FROM works")
	if err != nil {
		http.Error(w, "Query failed", 500)
		return
	}
	defer rows.Close()

	var works []Work
	for rows.Next() {
		var w Work
		var fromImport int
		rows.Scan(&w.ID, &w.Author, &w.Title, &w.ISBN, &w.Source, &fromImport)
		w.FromImport = fromImport == 1
		works = append(works, w)
	}
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Println(err)
		http.Error(w, "Error parsing template", 500)
		return
	}
	t.Execute(w, works)
}

func newWorkHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/new.html")
	if err != nil {
		log.Println(err)
		http.Error(w, "Template error", 500)
		return
	}
	t.Execute(w, nil)
}

func createWorkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	works := loadCatalog()

	id, _ := gonanoid.New()

	work := Work{
		ID:         id,
		Author:     r.FormValue("author"),
		Title:      r.FormValue("title"),
		ISBN:       r.FormValue("isbn"),
		Source:     "manual entry",
		FromImport: false,
	}

	works[work.ID] = work
	saveCatalog(works)
	http.Redirect(w, r, "/", http.StatusSeeOther)

}

func viewWorkHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/work/")
	works := loadCatalog()
	t, err := template.ParseFiles("templates/view.html")
	if err != nil {
		log.Println(err)
		http.Error(w, "Error parsing template", 500)
		return
	}
	if err := t.Execute(w, works[id]); err != nil {
		log.Println("Template execute error:", err)
	}
}

func editWorkHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/work/edit/")
	works := loadCatalog()
	t, err := template.ParseFiles("templates/edit.html")
	if err != nil {
		log.Println(err)
		http.Error(w, "Error parsing template", 500)
		return
	}
	if err := t.Execute(w, works[id]); err != nil {
		log.Println("Template execute error:", err)
	}
}

func updateWorkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/work/update/")

	works := loadCatalog()
	work, ok := works[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	log.Println("updating: ", work)

	work = works[id]

	work.Author = r.FormValue("author")
	work.Title = r.FormValue("title")
	work.ISBN = r.FormValue("isbn")

	works[id] = work
	saveCatalog(works)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func deleteWorkHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/work/delete/")
	_, err := db.Exec("DELETE FROM works WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Delete failed", 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, author, title, isbn, source, from_import FROM works")
	if err != nil {
		http.Error(w, "Query failed", 500)
		return
	}
	defer rows.Close()

	var works []Work
	for rows.Next() {
		var w Work
		var fromImport int
		rows.Scan(&w.ID, &w.Author, &w.Title, &w.ISBN, &w.Source, &fromImport)
		w.FromImport = fromImport == 1
		works = append(works, w)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=catalog_export.json")

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(works); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "Missing query", http.StatusBadRequest)
		return
	}

	openlibUrl := "https://openlibrary.org/search.json?q=" + url.QueryEscape(query)

	resp, err := http.Get(openlibUrl)
	if err != nil {
		http.Error(w, "Error requesting Book Search API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "OpenLibrary Book Search API returned error", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}

func loadCatalog() map[string]Work {
	if _, err := os.Stat(catalogFile); os.IsNotExist(err) {
		return map[string]Work{}
	}
	data, err := os.ReadFile(catalogFile)
	if err != nil {
		log.Println("Error reading catalog:", err)
		return map[string]Work{}
	}
	var works map[string]Work
	json.Unmarshal(data, &works)
	return works
}

func saveCatalog(works map[string]Work) {
	data, err := json.Marshal(works)
	if err != nil {
		log.Println("Error saving catalog:", err)
		return
	}
	os.WriteFile(catalogFile, data, 0644)
}

func importBooks() {
	data, err := os.ReadFile(importFile)
	if err != nil {
		log.Fatalf("Error reading file for import: %v", err)
		return
	}
	var ib map[string]ImportedWork
	json.Unmarshal(data, &ib)

	works := map[string]Work{}
	for _, v := range ib {
		b := Work{
			ID:         v.ID,
			Author:     v.Author,
			Title:      v.Title,
			ISBN:       v.ISBN,
			Source:     v.Source,
			FromImport: true,
		}

		works[b.ID] = b

	}
	saveCatalog(works)
}
