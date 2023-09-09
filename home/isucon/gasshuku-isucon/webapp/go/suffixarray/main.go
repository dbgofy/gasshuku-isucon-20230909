package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
)

type suffix struct {
	ID    string `db:"id"`
	Value string `db:"value"`
}

func main() {
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "3306")
	user := getEnvOrDefault("DB_USER", "isucon")
	pass := getEnvOrDefault("DB_PASS", "isucon")
	name := getEnvOrDefault("DB_NAME", "isulibrary")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Asia%%2FTokyo", user, pass, host, port, name)

	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	ctx := context.Background()

	var books []struct {
		ID     string `db:"id"`
		Title  string `db:"title"`
		Author string `db:"author"`
	}
	if err := db.SelectContext(ctx, &books, "SELECT id, title, author FROM `book`"); err != nil {
		log.Fatalln("select books", err)
	}

	var titles, authors []suffix
	for _, book := range books {
		title := []rune(book.Title)
		for i := 0; i < len(title); i++ {
			titles = append(titles, suffix{book.ID, string(title[i:])})
		}

		author := []rune(book.Author)
		for i := 0; i < len(author); i++ {
			authors = append(authors, suffix{book.ID, string(author[i:])})
		}
	}
	if _, err := db.NamedExecContext(ctx, "INSERT INTO `book_title_suffix` (`book_id`, `title_suffix`) VALUES (:id , :value)", titles); err != nil {
		log.Fatalln("insert titles", err)
	}
	if _, err := db.NamedExecContext(ctx, "INSERT INTO `book_author_suffix` (`book_id`, `author_suffix`) VALUES (:id , :value)", authors); err != nil {
		log.Fatalln("insert titles", err)
	}
}

func getEnvOrDefault(key string, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}

	return defaultValue
}
