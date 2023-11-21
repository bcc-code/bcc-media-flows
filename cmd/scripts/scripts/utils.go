package scripts

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

func GetParam(arg int, prompt string) string {
	var reader = bufio.NewReader(os.Stdin)
	if len(os.Args) > arg {
		return os.Args[arg]
	}
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	return strings.Split(text, "\n")[0]
}

func requireAdminClient() *resty.Client {
	adminUrl := os.Getenv("SCRIPTS_ADMIN_URL")
	adminToken := os.Getenv("SCRIPTS_ADMIN_TOKEN")

	if adminUrl == "" {
		adminUrl = "https://admin.brunstad.tv"
	}

	if adminToken == "" {
		panic("SCRIPTS_ADMIN_TOKEN env var is not set. Create an admin user in directus and assign a static token")
	}

	restyClient := resty.New()
	restyClient.SetBaseURL(adminUrl)
	restyClient.Header.Set("Authorization", "Bearer "+adminToken)
	restyClient.RetryCount = 3
	restyClient.RetryWaitTime = 5 * time.Second
	return restyClient
}

func requireSql() *sql.DB {
	var (
		host     = os.Getenv("SCRIPTS_DB_HOST")
		port     = os.Getenv("SCRIPTS_DB_PORT")
		user     = os.Getenv("SCRIPTS_DB_USER")
		password = os.Getenv("SCRIPTS_DB_PASSWORD")
		dbname   = os.Getenv("SCRIPTS_DB_NAME")
	)

	// Connection string
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	// Open the connection
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to db!")

	return db
}
