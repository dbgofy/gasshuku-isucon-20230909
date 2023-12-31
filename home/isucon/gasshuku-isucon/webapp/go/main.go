package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	"github.com/uptrace/opentelemetry-go-extra/otelsqlx"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/uptrace-go/uptrace"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()

	var revision string
	{
		info, _ := debug.ReadBuildInfo()
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				revision = s.Value
			}
		}
		uptrace.ConfigureOpentelemetry(
			uptrace.WithServiceName("webapp:" + revision),
		)
		defer uptrace.Shutdown(ctx)
	}

	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "3306")
	user := getEnvOrDefault("DB_USER", "isucon")
	pass := getEnvOrDefault("DB_PASS", "isucon")
	name := getEnvOrDefault("DB_NAME", "isulibrary")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Asia%%2FTokyo", user, pass, host, port, name)

	var err error
	db, err = otelsqlx.Open("mysql", dsn, otelsql.WithAttributes(semconv.DBSystemKey.String("mysql:"+revision)))
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	var key string
	err = db.Get(&key, "SELECT `key` FROM `key` WHERE `id` = (SELECT MAX(`id`) FROM `key`)")
	if err != nil {
		log.Panic(err)
	}

	block, err = aes.NewCipher([]byte(key))
	if err != nil {
		log.Panic(err)
	}

	e := echo.New()
	e.Debug = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		e.DefaultHTTPErrorHandler(err, c)
		go c.Logger().Errorj(log.JSON{
			"error":  err.Error(),
			"method": c.Request().Method,
			"path":   c.Path(),
			"params": c.QueryParams(),
		})
	}
	e.Use(otelecho.Middleware("dev-1"))

	api := e.Group("/api")
	{
		api.POST("/initialize", initializeHandler)

		membersAPI := api.Group("/members")
		{
			membersAPI.POST("", postMemberHandler)
			membersAPI.GET("", getMembersHandler)
			membersAPI.GET("/:id", getMemberHandler)
			membersAPI.PATCH("/:id", patchMemberHandler)
			membersAPI.DELETE("/:id", banMemberHandler)
			membersAPI.GET("/:id/qrcode", getMemberQRCodeHandler)
		}

		booksAPI := api.Group("/books")
		{
			booksAPI.POST("", postBooksHandler)
			booksAPI.GET("", getBooksHandler)
			booksAPI.GET("/:id", getBookHandler)
			booksAPI.GET("/:id/qrcode", getBookQRCodeHandler)
		}

		lendingsAPI := api.Group("/lendings")
		{
			lendingsAPI.POST("", postLendingsHandler)
			lendingsAPI.GET("", getLendingsHandler)
			lendingsAPI.POST("/return", returnLendingsHandler)
		}
	}

	e.Logger.Fatal(e.Start(":8080"))
}

/*
---------------------------------------------------------------
Domain Models
---------------------------------------------------------------
*/

// 会員
type Member struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Address     string    `json:"address" db:"address"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Banned      bool      `json:"banned" db:"banned"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// 図書分類
type Genre int

// 国際十進分類法に従った図書分類
const (
	General         Genre = iota // 総記
	Philosophy                   // 哲学・心理学
	Religion                     // 宗教・神学
	SocialScience                // 社会科学
	Vacant                       // 未定義
	Mathematics                  // 数学・自然科学
	AppliedSciences              // 応用科学・医学・工学
	Arts                         // 芸術
	Literature                   // 言語・文学
	Geography                    // 地理・歴史
)

// 蔵書
type Book struct {
	ID        string    `json:"id" db:"id"`
	Title     string    `json:"title" db:"title"`
	Author    string    `json:"author" db:"author"`
	Genre     Genre     `json:"genre" db:"genre"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// 貸出記録
type Lending struct {
	ID        string    `json:"id" db:"id"`
	MemberID  string    `json:"member_id" db:"member_id"`
	BookID    string    `json:"book_id" db:"book_id"`
	Due       time.Time `json:"due" db:"due"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

/*
---------------------------------------------------------------
Utilities
---------------------------------------------------------------
*/

// ULIDを生成
func generateID() string {
	return ulid.Make().String()
}

var db *sqlx.DB

func getEnvOrDefault(key string, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}

	return defaultValue
}

var (
	block              cipher.Block
	qrFileLock         sync.Mutex
	notBannedMemberNum atomic.Int32

	bookByGenreCache []*atomic.Int64
)

// AES + CTRモード + base64エンコードでテキストを暗号化
func encrypt(plainText string) (string, error) {
	cipherText := make([]byte, aes.BlockSize+len([]byte(plainText)))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	encryptStream := cipher.NewCTR(block, iv)
	encryptStream.XORKeyStream(cipherText[aes.BlockSize:], []byte(plainText))
	return base64.URLEncoding.EncodeToString(cipherText), nil
}

// AES + CTRモード + base64エンコードで暗号化されたテキストを複合
func decrypt(cipherText string) (string, error) {
	cipherByte, err := base64.URLEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	decryptedText := make([]byte, len([]byte(cipherByte[aes.BlockSize:])))
	decryptStream := cipher.NewCTR(block, []byte(cipherByte[:aes.BlockSize]))
	decryptStream.XORKeyStream(decryptedText, []byte(cipherByte[aes.BlockSize:]))
	return string(decryptedText), nil
}

// QRコードを生成
func generateQRCode(id string) ([]byte, error) {
	qrCodeFileName := fmt.Sprintf("../images/%s.png", id)
	file, err := os.Open(qrCodeFileName)
	if err == nil {
		defer file.Close()
		return io.ReadAll(file)
	}

	encryptedID, err := encrypt(id)
	if err != nil {
		return nil, err
	}

	qrFileLock.Lock()
	defer qrFileLock.Unlock()
	//TODO: 一旦直前でlockするようにする
	/*
		生成するQRコードの仕様
		 - PNGフォーマット
		 - QRコードの1モジュールは1ピクセルで表現
		 - バージョン6 (41x41ピクセル、マージン含め49x49ピクセル)
		 - エラー訂正レベルM (15%)
	*/
	err = exec.
		Command("sh", "-c", fmt.Sprintf("echo \"%s\" | qrencode -o %s -t PNG -s 1 -v 6 --strict-version -l M", encryptedID, qrCodeFileName)).
		Run()
	if err != nil {
		return nil, err
	}

	file, err = os.Open(qrCodeFileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

/*
---------------------------------------------------------------
Initialization API
---------------------------------------------------------------
*/

type InitializeHandlerRequest struct {
	Key string `json:"key"`
}

type InitializeHandlerResponse struct {
	Language string `json:"language"`
}

type genreCount struct {
	Genre Genre `db:"genre"`
	Count int64 `db:"c"`
}

// 初期化用ハンドラ
func initializeHandler(c echo.Context) error {
	var req InitializeHandlerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if len(req.Key) != 16 {
		return echo.NewHTTPError(http.StatusBadRequest, "key must be 16 characters")
	}

	g, ctx := errgroup.WithContext(c.Request().Context())

	g.Go(func() error {
		cmd := exec.CommandContext(ctx, "bash", "../sql/init_db.sh")
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		return err
	})

	g.Go(func() error {
		_, err := db.ExecContext(c.Request().Context(), "INSERT INTO `key` (`key`) VALUES (?)", req.Key)
		return err
	})

	g.Go(func() error {
		var err error
		block, err = aes.NewCipher([]byte(req.Key))
		if err != nil {
			log.Panic(err.Error())
		}
		return nil
	})

	g.Go(func() error {
		var total int32
		err := db.GetContext(c.Request().Context(), &total, "SELECT COUNT(*) FROM `member` WHERE `banned` = false")
		if err != nil {
			return err
		}
		notBannedMemberNum.Store(total)
		return nil
	})

	g.Go(func() error {
		var genreCounts []genreCount
		err := db.SelectContext(c.Request().Context(), &genreCounts, "SELECT genre, count(1) as c FROM `book` GROUP BY genre order by genre")
		if err != nil {
			return err
		}
		bookByGenreCache = make([]*atomic.Int64, 10)
		for _, genreCount := range genreCounts {
			bookByGenreCache[genreCount.Genre] = new(atomic.Int64)
			bookByGenreCache[genreCount.Genre].Store(genreCount.Count)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, InitializeHandlerResponse{
		Language: "Go",
	})
}

/*
---------------------------------------------------------------
Members API
---------------------------------------------------------------
*/

type PostMemberRequest struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
}

// 会員登録
func postMemberHandler(c echo.Context) error {
	var req PostMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" || req.Address == "" || req.PhoneNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name, address, phoneNumber are required")
	}

	id := generateID()

	res := Member{
		ID:          id,
		Name:        req.Name,
		Address:     req.Address,
		PhoneNumber: req.PhoneNumber,
		Banned:      false,
		CreatedAt:   time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Truncate(time.Microsecond),
	}
	_, err := db.ExecContext(c.Request().Context(),
		"INSERT INTO `member` (`id`, `name`, `address`, `phone_number`, `banned`, `created_at`) VALUES (?, ?, ?, ?, false, ?)",
		res.ID, res.Name, res.Address, res.PhoneNumber, res.CreatedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	notBannedMemberNum.Add(1)

	return c.JSON(http.StatusCreated, res)
}

const memberPageLimit = 100

type GetMembersResponse struct {
	Members []Member `json:"members"`
	Total   int      `json:"total"`
}

// 会員一覧を取得 (ページネーションあり)
func getMembersHandler(c echo.Context) error {
	var err error

	lastMemberID := c.QueryParam("last_member_id")

	order := c.QueryParam("order")
	if order != "" && order != "name_asc" && order != "name_desc" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order")
	}

	var lastMemberName string
	if lastMemberID != "" && (order == "name_asc" || order == "name_desc") {
		err = db.GetContext(c.Request().Context(), &lastMemberName, "SELECT `name` FROM `member` WHERE `id` = ?", lastMemberID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	query := "SELECT * FROM `member` WHERE `banned` = false "
	var filterString string
	switch order {
	case "name_asc":
		filterString = lastMemberName
		if filterString == "" {
			query += "ORDER BY `name` ASC "
		} else {
			query += "AND `name` > ? ORDER BY `name` ASC "
		}
	case "name_desc":
		filterString = lastMemberName
		if filterString == "" {
			query += "ORDER BY `name` DESC "
		} else {
			query += "AND `name` < ? ORDER BY `name` DESC "
		}
	default:
		filterString = lastMemberID
		if filterString == "" {
			query += "ORDER BY `id` ASC "
		} else {
			query += "AND `id` > ? ORDER BY `id` ASC "
		}
	}
	query += "LIMIT ?"

	members := []Member{}
	if filterString == "" {
		err = db.SelectContext(c.Request().Context(), &members, query, memberPageLimit)
	} else {
		err = db.SelectContext(c.Request().Context(), &members, query, filterString, memberPageLimit)
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if len(members) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no members to show in this page")
	}

	return c.JSON(http.StatusOK, GetMembersResponse{
		Members: members,
		Total:   int(notBannedMemberNum.Load()),
	})
}

// 会員を取得
func getMemberHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	encrypted := c.QueryParam("encrypted")
	if encrypted == "true" {
		var err error
		id, err = decrypt(id)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	} else if encrypted != "" && encrypted != "false" {
		return echo.NewHTTPError(http.StatusBadRequest, "encrypted must be boolean value")
	}

	member := Member{}
	err := db.GetContext(c.Request().Context(), &member, "SELECT * FROM `member` WHERE `id` = ? AND `banned` = false", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, member)
}

type PatchMemberRequest struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
}

// 会員情報編集
func patchMemberHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	var req PatchMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" && req.Address == "" && req.PhoneNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name, address or phoneNumber is required")
	}

	tx, err := db.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 会員の存在を確認
	err = tx.GetContext(c.Request().Context(), &Member{}, "SELECT * FROM `member` WHERE `id` = ? AND `banned` = false", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	query := "UPDATE `member` SET "
	params := []any{}
	if req.Name != "" {
		query += "`name` = ?, "
		params = append(params, req.Name)
	}
	if req.Address != "" {
		query += "`address` = ?, "
		params = append(params, req.Address)
	}
	if req.PhoneNumber != "" {
		query += "`phone_number` = ?, "
		params = append(params, req.PhoneNumber)
	}
	query = strings.TrimSuffix(query, ", ")
	query += " WHERE `id` = ?"
	params = append(params, id)

	_, err = tx.ExecContext(c.Request().Context(), query, params...)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_ = tx.Commit()

	return c.NoContent(http.StatusNoContent)
}

// 会員をBAN
func banMemberHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	tx, err := db.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 会員の存在を確認
	err = tx.GetContext(c.Request().Context(), &Member{}, "SELECT * FROM `member` WHERE `id` = ? AND `banned` = false", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_, err = tx.ExecContext(c.Request().Context(), "UPDATE `member` SET `banned` = true WHERE `id` = ?", id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_, err = tx.ExecContext(c.Request().Context(), "DELETE FROM `lending` WHERE `member_id` = ?", id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	notBannedMemberNum.Add(-1)
	_ = tx.Commit()

	return c.NoContent(http.StatusNoContent)
}

// 会員証用のQRコードを取得
func getMemberQRCodeHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	// 会員の存在確認
	err := db.GetContext(c.Request().Context(), &Member{}, "SELECT * FROM `member` WHERE `id` = ? AND `banned` = false", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	qrCode, err := generateQRCode(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Blob(http.StatusOK, "image/png", qrCode)
}

/*
---------------------------------------------------------------
Books API
---------------------------------------------------------------
*/

type PostBooksRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Genre  Genre  `json:"genre"`
}

type bookTitleSuffix struct {
	BookID      string `db:"book_id"`
	TitleSuffix string `db:"title_suffix"`
}

type bookAuthorSuffix struct {
	BookID       string `db:"book_id"`
	AuthorSuffix string `db:"author_suffix"`
}

// 蔵書を登録 (複数札を一気に登録)
func postBooksHandler(c echo.Context) error {
	var reqSlice []PostBooksRequest
	if err := c.Bind(&reqSlice); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	createdAt := time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Truncate(time.Microsecond)

	books := make([]Book, 0, len(reqSlice))
	for _, req := range reqSlice {
		if req.Title == "" || req.Author == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "title, author is required")
		}
		if req.Genre < 0 || req.Genre > 9 {
			return echo.NewHTTPError(http.StatusBadRequest, "genre is invalid")
		}
		books = append(books, Book{
			ID:        generateID(),
			Title:     req.Title,
			Author:    req.Author,
			Genre:     req.Genre,
			CreatedAt: createdAt,
		})
	}

	bookTitleSuffixes := make([]bookTitleSuffix, 0, len(books))
	bookAuthorSuffixes := make([]bookAuthorSuffix, 0, len(books))
	for _, book := range books {
		title := []rune(book.Title)
		for i := 0; i < len(title); i++ {
			bookTitleSuffixes = append(bookTitleSuffixes, bookTitleSuffix{
				BookID:      book.ID,
				TitleSuffix: string(title[i:]),
			})
		}
		author := []rune(book.Author)
		for i := 0; i < len(author); i++ {
			bookAuthorSuffixes = append(bookAuthorSuffixes, bookAuthorSuffix{
				BookID:       book.ID,
				AuthorSuffix: string(author[i:]),
			})
		}
	}
	tx, err := db.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()
	// bulk insert
	_, err = db.NamedExecContext(c.Request().Context(), "INSERT INTO `book` (`id`, `title`, `author`, `genre`, `created_at`) VALUES (:id , :title , :author , :genre , :created_at)", books)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	_, err = db.NamedExecContext(c.Request().Context(), "INSERT INTO `book_title_suffix` (`book_id`, `title_suffix`) VALUES (:book_id , :title_suffix)", bookTitleSuffixes)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	_, err = db.NamedExecContext(c.Request().Context(), "INSERT INTO `book_author_suffix` (`book_id`, `author_suffix`) VALUES (:book_id , :author_suffix)", bookAuthorSuffixes)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	for _, req := range reqSlice {
		bookByGenreCache[req.Genre].Add(1)
	}

	_ = tx.Commit()

	return c.JSON(http.StatusCreated, books)
}

const bookPageLimit = 50

type GetBooksResponse struct {
	Books []GetBookResponse `json:"books"`
	Total int               `json:"total"`
}

// 蔵書を検索
func getBooksHandler(c echo.Context) error {
	title := c.QueryParam("title")
	author := c.QueryParam("author")
	genre := c.QueryParam("genre")
	if genre != "" {
		genreInt, err := strconv.Atoi(genre)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if genreInt < 0 || genreInt > 9 {
			return echo.NewHTTPError(http.StatusBadRequest, "genre is invalid")
		}
	}
	if genre == "" && title == "" && author == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title, author or genre is required")
	}

	pageStr := c.QueryParam("page")
	if pageStr == "" {
		pageStr = "1"
	}

	tx, err := db.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT COUNT(*) FROM `book` WHERE "
	var args []any
	if genre != "" {
		query += "genre = ? AND "
		args = append(args, genre)
	}
	if title != "" {
		query += "id in (SELECT book_id from book_title_suffix WHERE title_suffix LIKE ? ) AND "
		args = append(args, title+"%")
	}
	if author != "" {
		query += "id in (SELECT book_id from book_author_suffix WHERE author_suffix LIKE ? ) AND "
		args = append(args, author+"%")
	}
	query = strings.TrimSuffix(query, "AND ")

	var total int
	if genre != "" && title == "" && author == "" {
		genreInt, err := strconv.Atoi(genre)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		total = int(bookByGenreCache[Genre(genreInt)].Load())
	} else {
		err = tx.GetContext(c.Request().Context(), &total, query, args...)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	if total == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no books found")
	}

	query = strings.ReplaceAll(query, "COUNT(*)", "*")
	lastBookID := c.QueryParam("last_book_id")
	if lastBookID != "" {
		query += "AND `id` > ? "
		args = append(args, lastBookID)
	}
	query += "ORDER BY `id` ASC LIMIT ? "
	args = append(args, bookPageLimit)

	var books []Book
	err = tx.SelectContext(c.Request().Context(), &books, query, args...)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if len(books) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no books to show in this page")
	}

	res := GetBooksResponse{
		Books: make([]GetBookResponse, len(books)),
		Total: total,
	}
	bookIDs := make([]string, 0, len(books))
	for _, book := range books {
		bookIDs = append(bookIDs, book.ID)
	}
	query, args, err = sqlx.In("SELECT book_id FROM `lending` WHERE `book_id` IN (?)", bookIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	query = db.Rebind(query)

	var lendingBookIDs []string
	err = tx.SelectContext(c.Request().Context(), &lendingBookIDs, query, args...)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	resBookIDsMap := make(map[string]struct{}, len(lendingBookIDs))
	for _, resBookID := range lendingBookIDs {
		resBookIDsMap[resBookID] = struct{}{}
	}
	for i, book := range books {
		res.Books[i].Book = book

		_, ok := resBookIDsMap[book.ID]
		if ok {
			res.Books[i].Lending = true
		} else {
			res.Books[i].Lending = false
		}
	}

	_ = tx.Commit()

	return c.JSON(http.StatusOK, res)
}

type GetBookResponse struct {
	Book
	Lending bool `json:"lending"`
}

// 蔵書を取得
func getBookHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	encrypted := c.QueryParam("encrypted")
	if encrypted == "true" {
		var err error
		id, err = decrypt(id)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	} else if encrypted != "" && encrypted != "false" {
		return echo.NewHTTPError(http.StatusBadRequest, "encrypted must be boolean value")
	}

	tx, err := db.BeginTxx(c.Request().Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()

	book := Book{}
	err = tx.GetContext(c.Request().Context(), &book, "SELECT * FROM `book` WHERE `id` = ?", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	res := GetBookResponse{
		Book: book,
	}
	err = tx.GetContext(c.Request().Context(), &Lending{}, "SELECT * FROM `lending` WHERE `book_id` = ?", id) //TODO: LeftJoinで一回でいけそう
	if err == nil {
		res.Lending = true
	} else if errors.Is(err, sql.ErrNoRows) {
		res.Lending = false
	} else {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_ = tx.Commit()

	return c.JSON(http.StatusOK, res)
}

// 蔵書のQRコードを取得
func getBookQRCodeHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	// 蔵書の存在確認
	err := db.GetContext(c.Request().Context(), &Book{}, "SELECT * FROM `book` WHERE `id` = ?", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	qrCode, err := generateQRCode(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Blob(http.StatusOK, "image/png", qrCode)
}

/*
---------------------------------------------------------------
Lending API
---------------------------------------------------------------
*/

// 貸出期間(ミリ秒)
const LendingPeriod = 3000

type PostLendingsRequest struct {
	BookIDs  []string `json:"book_ids"`
	MemberID string   `json:"member_id"`
}

type PostLendingsResponse struct {
	Lending
	MemberName string `json:"member_name"`
	BookTitle  string `json:"book_title"`
}

// 本を貸し出し
func postLendingsHandler(c echo.Context) error {
	var req PostLendingsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.MemberID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "member_id is required")
	}
	if len(req.BookIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one book_ids is required")
	}

	tx, err := db.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 会員の存在確認
	var member Member
	err = tx.GetContext(c.Request().Context(), &member, "SELECT * FROM `member` WHERE `id` = ?", req.MemberID) //TODO: お前Tx内でやる必要なくね
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lendingTime := time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Truncate(time.Microsecond)
	due := lendingTime.Add(LendingPeriod * time.Millisecond) //MEMO: created_atから算出できるので持つ必要なさそう？
	res := make([]PostLendingsResponse, len(req.BookIDs))

	for i, bookID := range req.BookIDs {
		// 蔵書の存在確認
		var book Book
		err = tx.GetContext(c.Request().Context(), &book, "SELECT * FROM `book` WHERE `id` = ?", bookID) //TODO: お前もTx内でやる必要ないよね。あとIN使え。
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return echo.NewHTTPError(http.StatusNotFound, err.Error())
			}

			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		// 貸し出し中かどうか確認
		var lending Lending
		err = tx.GetContext(c.Request().Context(), &lending, "SELECT * FROM `lending` WHERE `book_id` = ?", bookID)
		if err == nil {
			return echo.NewHTTPError(http.StatusConflict, "this book is already lent")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		id := generateID()

		// 貸し出し
		_, err = tx.ExecContext(c.Request().Context(),
			"INSERT INTO `lending` (`id`, `book_id`, `member_id`, `due`, `created_at`) VALUES (?, ?, ?, ?, ?)", //TODO: bulkInsert
			id, bookID, req.MemberID, due, lendingTime)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		err := tx.GetContext(c.Request().Context(), &res[i], "SELECT * FROM `lending` WHERE `id` = ?", id) //TODO: だから必要ないやろお前
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		res[i].MemberName = member.Name
		res[i].BookTitle = book.Title
	}

	_ = tx.Commit()

	return c.JSON(http.StatusCreated, res)
}

type GetLendingsResponse struct {
	Lending
	MemberName string `json:"member_name"`
	BookTitle  string `json:"book_title"`
}

type GetLendingsHandlerQuery struct {
	ID         string    `db:"lending_id"`
	MemberID   string    `db:"member_id"`
	BookID     string    `db:"book_id"`
	Due        time.Time `db:"due"`
	CreatedAt  time.Time `db:"created_at"`
	MemberName string    `db:"member_name"`
	BookTitle  string    `db:"book_title"`
}

func getLendingsHandler(c echo.Context) error {
	overDue := c.QueryParam("over_due")
	if overDue != "" && overDue != "true" && overDue != "false" {
		return echo.NewHTTPError(http.StatusBadRequest, "over_due must be boolean value")
	}

	query := "SELECT " +
		"`lending`.`id` as `lending_id`, " +
		"`lending`.`member_id` as `member_id`, " +
		"`lending`.`book_id` as `book_id`, " +
		"`lending`.`due` as `due`, " +
		"`lending`.`created_at` as `created_at`, " +
		"`member`.`name` as `member_name`, " +
		"`book`.`title` as `book_title` " +
		" FROM `lending` INNER JOIN `member` ON `lending`.`member_id` = `member`.`id` INNER JOIN `book` ON `lending`.`book_id` = `book`.`id` "
	args := []any{}
	if overDue == "true" {
		query += " WHERE `due` > ?"
		args = append(args, time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Truncate(time.Microsecond))
	}
	query += " ORDER BY `lending`.`id` ASC"

	var lendings []GetLendingsHandlerQuery
	err := db.SelectContext(c.Request().Context(), &lendings, query, args...)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	res := make([]GetLendingsResponse, len(lendings))
	for i, lending := range lendings {
		res[i] = GetLendingsResponse{
			Lending: Lending{
				ID:        lending.ID,
				MemberID:  lending.MemberID,
				BookID:    lending.BookID,
				Due:       lending.Due,
				CreatedAt: lending.CreatedAt,
			},
			MemberName: lending.MemberName,
			BookTitle:  lending.BookTitle,
		}
	}

	return c.JSON(http.StatusOK, res)
}

type ReturnLendingsRequest struct {
	BookIDs  []string `json:"book_ids"`
	MemberID string   `json:"member_id"`
}

// 蔵書を返却
func returnLendingsHandler(c echo.Context) error {
	var req ReturnLendingsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.MemberID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "member_id is required")
	}
	if len(req.BookIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one book_ids is required")
	}

	tx, err := db.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 会員の存在確認
	err = tx.GetContext(c.Request().Context(), &Member{}, "SELECT * FROM `member` WHERE `id` = ?", req.MemberID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	for _, bookID := range req.BookIDs {
		// 貸し出しの存在確認
		var lending Lending
		err = tx.GetContext(c.Request().Context(), &lending,
			"SELECT * FROM `lending` WHERE `member_id` = ? AND `book_id` = ?", req.MemberID, bookID) //TODO: 消してみてダメだったらnotFoundを返せばいいので、これいらないはず
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return echo.NewHTTPError(http.StatusNotFound, err.Error())
			}

			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		_, err = tx.ExecContext(c.Request().Context(),
			"DELETE FROM `lending` WHERE `member_id` =? AND `book_id` =?", req.MemberID, bookID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	_ = tx.Commit()

	return c.NoContent(http.StatusNoContent)
}
