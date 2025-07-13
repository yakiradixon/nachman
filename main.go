package main

import (
    "encoding/json"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "sync"
    "time"
)

type Book struct {
    ID       int64 `json:"id"`
    Author   string `json:"author"`
    Title    string `json:"title"`
    ISBN     string `json:"isbn"`
    Tags     string `json:"tags"`
    Notes    string `json:"notes"`
    Source   string `json:"source"`
}

var (
    catalogFile = "catalog.json"
    mu          sync.Mutex
)

func main() {
    http.HandleFunc("/", formHandler)
    http.HandleFunc("/add", addHandler)
    log.Println("Server started at http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func formHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := `
    <html>
    <head><title>Book Catalog</title></head>
    <body>
        <h1>Manual Entry</h1>
        <form action="/add" method="POST">
            Title: <input name="title" required><br>
            Author: <input name="author"><br>
            ISBN: <input name="isbn"><br>
            Tags: <textarea name="tags"></textarea><br>
            <p> Separate tags with commas, like "foo, bar, baz"</p>
            Notes: <textarea name="notes"></textarea><br>
            <input type="submit" value="Add Book">
        </form>
        <h2>Catalog</h2>
        <ul>
        {{range .}}
            <li><p>Title: <b>{{.Title}}</b></p>
                <p>Author: {{.Author}}</p>
                <p>ISBN: {{.ISBN}}</p>
                <p>Tags: {{.Tags}} </p>
                <p>Notes: {{.Notes}}</p>
            </li>
        {{end}}
        </ul>
    </body>
    </html>
    `
    books := loadCatalog()
    t := template.Must(template.New("form").Parse(tmpl))
    t.Execute(w, books)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Redirect(w, r, "/", http.StatusSeeOther)
        return
    }

    book := Book{
        ID: time.Now().UnixNano(),
        Author: r.FormValue("author"),
        Title:  r.FormValue("title"),
        ISBN:   r.FormValue("isbn"),
        Tags:  r.FormValue("tags"),
        Notes:  r.FormValue("notes"),
        Source: "manual entry",
    }
    mu.Lock()
    books := loadCatalog()
    books = append(books, book)
    saveCatalog(books)
    mu.Unlock()
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

func loadCatalog() []Book {
    if _, err := os.Stat(catalogFile); os.IsNotExist(err) {
        return []Book{}
    }
    data, err := ioutil.ReadFile(catalogFile)
    if err != nil {
        log.Println("Error reading catalog:", err)
        return []Book{}
    }
    var books []Book
    json.Unmarshal(data, &books)
    return books
}

func saveCatalog(books []Book) {
    data, err := json.MarshalIndent(books, "", "  ")
    if err != nil {
        log.Println("Error saving catalog:", err)
        return
    }
    ioutil.WriteFile(catalogFile, data, 0644)
}
