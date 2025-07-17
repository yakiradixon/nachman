package main

import (
    "encoding/json"
    // "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "strconv"
    "sync"
    "time"
)

type Book struct {
    ID       string `json:"id"`
    Author   string `json:"author"`
    Title    string `json:"title"`
    ISBN     string `json:"isbn"`
    Source   string `json:"source"`
    FromImport bool
}

type ImportedBook struct {
    ID       string `json:"books_id"`
    Author   string `json:"primaryauthor"`
    Title    string `json:"title"`
    // ISBN     map[string]string `json:"isbn"`
    ISBN     string `json:"originalisbn"`
    Source   string `json:"source"`
    FromImport  bool
}

var (
    catalogFile = "catalog.json"
    importFile = "import.json"
    mu          sync.Mutex
)

func main() {
    importBooks()
    http.HandleFunc("/", formHandler)
    http.HandleFunc("/add", addHandler)
    http.HandleFunc("/work/new", newWorkHandler)
    http.HandleFunc("/work/create", createWorkHandler)
    http.HandleFunc("/export", exportHandler)

    log.Println("Server started at http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func formHandler(w http.ResponseWriter, r *http.Request) {
    books := loadCatalog()
    t, err := template.ParseFiles("templates/index.html")
    if err != nil {
        log.Println(err)
        http.Error(w, "Error parsing template", 500)
        return
    }
    t.Execute(w, books)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Redirect(w, r, "/", http.StatusSeeOther)
        return
    }

    book := Book{
        ID: strconv.FormatInt(time.Now().UnixNano(), 10),
        Author: r.FormValue("author"),
        Title:  r.FormValue("title"),
        ISBN:   r.FormValue("isbn"),
        Source: "manual entry",
        FromImport: false,
    }


    mu.Lock()
    books := loadCatalog()
    books[book.ID] = book
    saveCatalog(books)
    mu.Unlock()
    http.Redirect(w, r, "/", http.StatusSeeOther)
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

    books := loadCatalog()

    b := Book{
        ID:         strconv.FormatInt(time.Now().UnixNano(), 10),
        Author:     r.FormValue("author"),
        Title:      r.FormValue("title"),
        ISBN:       r.FormValue("isbn"),
        Source:     "manual entry",
        FromImport: false,
    }

    books[b.ID] = b
    saveCatalog(books)
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

func exportHandler(w http.ResponseWriter, r *http.Request) {

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Content-Disposition", "attachment; filename=catalog_export.json")

    encoder := json.NewEncoder(w)
    encoder.SetIndent("", "  ") // Pretty printing
    works := loadCatalog()
    if err := encoder.Encode(works); err != nil {
        http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
    }
}


func loadCatalog() map[string]Book {
    if _, err := os.Stat(catalogFile); os.IsNotExist(err) {
        return map[string]Book{}
    }
    data, err := os.ReadFile(catalogFile)
    if err != nil {
        log.Println("Error reading catalog:", err)
        return map[string]Book{}
    }
    var books map[string]Book
    json.Unmarshal(data, &books)
    return books
}

func saveCatalog(books map[string]Book) {
    data, err := json.Marshal(books)
    //data, err := json.MarshalIndent(books, "", "  ")
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
    var ib map[string]ImportedBook
    json.Unmarshal(data, &ib)


    books := map[string]Book{}
    for _, v := range ib {
        b := Book{
            ID: v.ID,
            Author: v.Author,
            Title: v.Title,
            ISBN: v.ISBN,
            Source: v.Source,
            FromImport: true,
        }

        books[b.ID] = b

    }
    saveCatalog(books)
}