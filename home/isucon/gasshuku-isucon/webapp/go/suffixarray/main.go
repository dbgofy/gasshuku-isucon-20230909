package main

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

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

	size := 100
	titles := make([]map[string]interface{}, 0, size)
	authors := make([]map[string]interface{}, 0, size)
	for _, book := range books {
		title := []rune(book.Title)
		for i := 0; i < len(title); i++ {
			titles = append(titles, map[string]interface{}{
				"book_id":      book.ID,
				"title_suffix": string(title[i:]),
			})
		}
		if len(titles)%size == 0 {
			insert(ctx, db, "title", titles)
			titles = make([]map[string]interface{}, 0, size)
		}

		author := []rune(book.Author)
		for i := 0; i < len(author); i++ {
			authors = append(authors, map[string]interface{}{
				"book_id":       book.ID,
				"author_suffix": string(author[i:]),
			})
		}
		if len(authors)%size == 0 {
			insert(ctx, db, "author", authors)
			authors = make([]map[string]interface{}, 0, size)
		}
	}
	insert(ctx, db, "title", titles)
	insert(ctx, db, "author", authors)
}

func insert(ctx context.Context, db *sqlx.DB, column string, values []map[string]interface{}) error {
	query := "INSERT INTO `book_" + column + "_suffix` (`book_id`, `" + column + "_suffix`) VALUES (:book_id , :" + column + "_suffix)"
	log.Println(query, values[0])
	_, err := db.NamedExecContext(ctx, query, values)
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
