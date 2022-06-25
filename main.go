package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

//basic - middleware
func logging(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		f(w, r)
	}
}

func foo(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "foo")
}

func bar(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "bar")
}

// Sessions

var (
	key   = []byte("clau-super-secreta")
	store = sessions.NewCookieStore(key)
)

func secret(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "sweet-cookie")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Error(w, "Prohibit!", http.StatusForbidden)
		return
	}

	fmt.Fprintln(w, "El pastís es molt bó!")
}

func login(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "sweet-cookie")

	session.Values["authenticated"] = true
	session.Save(r, w)
}

func logout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "sweet-cookie")

	session.Values["authenticated"] = false
	session.Save(r, w)
}

// encode - decode JSON

type User struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Age       int    `json:"age"`
}

func main() {

	db, err := sql.Open("mysql", "sakila:123456@(127.0.0.1:3306)/test1?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	/*{ // crear nova taula
		query := `
		CREATE TABLE users (
			id INT AUTO_INCREMENT,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (id)
		);`

		if _, err := db.Exec(query); err != nil {
			log.Fatal(err)
		}

	}*/

	r := mux.NewRouter()

	//basic - middleware
	r.HandleFunc("/foo", logging(foo))
	r.HandleFunc("/bar", logging(bar))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// inici
		type Todo struct {
			Title string
			Done  bool
		}

		type TodoPageData struct {
			PageTitle string
			Todos     []Todo
		}
		tmpl, err := template.ParseFiles("index.html")

		if err != nil {
			log.Fatal(err)
		}
		data := TodoPageData{
			PageTitle: "LLista coses a fer",
			Todos: []Todo{
				{Title: "Tasca 1", Done: false},
				{Title: "Tasca 2", Done: true},
				{Title: "Tasca 3", Done: false},
			},
		}

		tmpl.Execute(w, data)

	})

	// sessions

	r.HandleFunc("/secret", secret)
	r.HandleFunc("/login", login)
	r.HandleFunc("/logout", logout)

	r.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		type ContactDetails struct {
			Email   string
			Subject string
			Message string
		}

		tmpl, err := template.ParseFiles("form.html")

		if err != nil {
			log.Fatal(err)
		}

		if r.Method != http.MethodPost {
			tmpl.Execute(w, nil)
			return
		}

		details := ContactDetails{
			Email:   r.FormValue("email"),
			Subject: r.FormValue("subject"),
			Message: r.FormValue("message"),
		}

		_ = details

		tmpl.Execute(w, struct{ Success bool }{true})
	})

	r.HandleFunc("/create/user/{username}/pass/{password}", func(w http.ResponseWriter, r *http.Request) {
		// inserir nou usuari
		vars := mux.Vars(r)
		user := vars["username"]
		pass := vars["password"]
		created := time.Now()

		result, err := db.Exec(`INSERT INTO users(username, password, created_at) VALUES (?,?,?)`, user, pass, created)
		if err != nil {
			log.Fatal(err)
		}

		id, err := result.LastInsertId()
		fmt.Fprintf(w, "se ha creado el usuario con id: %d\n", id)

	})

	r.HandleFunc("/query/id/{code}", func(w http.ResponseWriter, r *http.Request) {
		// query usuaris per id
		vars := mux.Vars(r)
		code := vars["code"]

		var (
			id         int
			username   string
			password   string
			created_at time.Time
		)

		query := "SELECT id, username, password, created_at FROM users WHERE id = ?"
		if err := db.QueryRow(query, code).Scan(&id, &username, &password, &created_at); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(w, "Id: %d\n Usuari: %s\n Contrasenya: %s\n Creat el: %d\n", id, username, password, created_at)
	})

	r.HandleFunc("/query/all", func(w http.ResponseWriter, r *http.Request) {
		// query tots els usuaris
		type user struct {
			id         int
			username   string
			password   string
			created_at time.Time
		}

		rows, err := db.Query("SELECT id, username, password, created_at FROM users")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		var users []user
		for rows.Next() {
			var u user
			if err := rows.Scan(&u.id, &u.username, &u.password, &u.created_at); err != nil {
				log.Fatal(err)
			}
			users = append(users, u)
			fmt.Fprintf(w, "Id: %d\n Usuari: %s\n Contrasenya: %s\n Creat el: %d\n", u.id, u.username, u.password, u.created_at)

		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}
	})

	r.HandleFunc("/delete/{id}", func(w http.ResponseWriter, r *http.Request) {
		// borrar usuari per id
		vars := mux.Vars(r)
		id := vars["id"]
		_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprintf(w, "Se ha borrado el usuario con id: %s con exito!", id)
	})

	// encode decode JSON

	r.HandleFunc("/decode", func(w http.ResponseWriter, r *http.Request) {
		var user User
		json.NewDecoder(r.Body).Decode(&user)
		fmt.Fprintf(w, "%s %s te %d anys!", user.Firstname, user.Lastname, user.Age)
	})

	r.HandleFunc("/encode", func(w http.ResponseWriter, r *http.Request) {
		Isam := User{
			Firstname: "Isam",
			Lastname:  "Hajji",
			Age:       23,
		}

		json.NewEncoder(w).Encode(Isam)
	})
	// serveix arxius i imatges (sense mux router)
	/*fs := http.FileServer(http.Dir("assets/"))

	http.Handle("/static/", http.StripPrefix("/static/", fs))*/

	// serveix arxius i imatges (mux router)
	staticDir := "/assets/"

	r.PathPrefix(staticDir).Handler(http.StripPrefix(staticDir, http.FileServer(http.Dir("."+staticDir))))

	http.ListenAndServe(":80", r)
}
