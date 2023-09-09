package main

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
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

	size := 3000
	titles := make([]suffix, 0, size)
	authors := make([]suffix, 0, size)
	for j, book := range books {
		title := []rune(book.Title)
		for i := 0; i < len(title); i++ {
			titles = append(titles, suffix{book.ID, string(title[i:])})
		}
		if len(titles)%size == 0 {
			insert(ctx, db, "title", titles)
			titles = make([]suffix, 0, size)
		}

		author := []rune(book.Author)
		for i := 0; i < len(author); i++ {
			authors = append(authors, suffix{book.ID, string(author[i:])})
		}
		if len(authors)%size == 0 {
			insert(ctx, db, "author", authors)
			authors = make([]suffix, 0, size)
		}

		if j%(len(books)/100) == 0 {
			log.Printf("books %d%% (%d/%d)\n", j%(len(books)/100), j, len(books))
		}
	}
	insert(ctx, db, "title", titles)
	insert(ctx, db, "author", authors)
}

func insert(ctx context.Context, db *sqlx.DB, column string, values []suffix) error {
	log.Printf("insert %d values into %s\n", len(values), column)
	_, err := db.NamedExecContext(ctx, "INSERT INTO `book_"+column+"_suffix` (`book_id`, `"+column+"_suffix`) VALUES (:id , :value)", values)
	if err != nil {
		return fmt.Errorf("insert %s: %w", column, err)
	}
	return nil
}

func getEnvOrDefault(key string, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}

	return defaultValue
}
